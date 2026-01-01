package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	indexingdb "github.com/gtsteffaniak/filebrowser/backend/database/indexing"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	RealPathCache = cache.NewCache[string](48*time.Hour, 72*time.Hour)
	IsDirCache    = cache.NewCache[bool](48*time.Hour, 72*time.Hour)
)

// actionConfig holds all configuration options for indexing operations
type actionConfig struct {
	Quick         bool // whether to perform a quick scan (skip unchanged directories)
	Recursive     bool // whether to recursively index subdirectories
	CheckViewable bool // whether to check if the path has viewable:true (for API access checks)
	IsRoutineScan bool // whether this is a routine/scheduled scan (vs initial indexing)
}

// ScannerInfo is the exposed scanner information for the client
type ScannerInfo struct {
	Stats
	Path            string `json:"path"`
	IsRoot          bool   `json:"isRoot"`
	CurrentSchedule int    `json:"currentSchedule"`
}

type Stats struct {
	NumDirs         uint64    `json:"numDirs"`
	NumFiles        uint64    `json:"numFiles"`
	NumDeleted      uint64    `json:"numDeleted"`
	QuickScanTime   int       `json:"quickScanDurationSeconds"`
	FullScanTime    int       `json:"fullScanDurationSeconds"`
	LastIndexedUnix int64     `json:"lastIndexedUnixTime"`
	Complexity      uint      `json:"complexity"`
	LastScanned     time.Time `json:"lastScanned"`
	DiskUsed        uint64    `json:"used"`
}

// reduced index is json exposed to the client
type ReducedIndex struct {
	Stats
	IdxName   string         `json:"name"`
	DiskTotal uint64         `json:"total"`
	Status    IndexStatus    `json:"status"`
	Scanners  []*ScannerInfo `json:"scanners,omitempty"`
}

type Index struct {
	ReducedIndex
	settings.Source  `json:"-"`
	db               *dbsql.IndexDB
	previousNumDirs  uint64              // Track previous NumDirs to use when scan in progress (computed value is 0)
	previousNumFiles uint64              // Track previous NumFiles to use when scan in progress (computed value is 0)
	scanners         map[string]*Scanner // path -> scanner
	mock             bool
	mu               sync.RWMutex
	childScanMutex   sync.Mutex // Serializes child scanner execution (only one child scanner runs at a time)
	// In-memory folder size tracking (not stored in SQLite)
	folderSizes   map[string]uint64 // path -> size (in-memory only, calculated from children)
	folderSizesMu sync.RWMutex      // Dedicated RWMutex allows concurrent reads, serializes writes
	// Scan session tracking: timestamp when scan session started (for timestamp-based conflict detection)
	scanSessionStartTime int64           // Unix timestamp when current scan session started (0 if no active scan)
	scanUpdatedPaths     map[string]bool // Tracks directories updated by the scan (to distinguish from API updates)
}

var (
	indexes         map[string]*Index
	indexesMutex    sync.RWMutex
	indexDB         *dbsql.IndexDB      // Shared database for all indexes
	indexingStorage *indexingdb.Storage // Persistent storage for index metadata
)

type IndexStatus string

const (
	READY       IndexStatus = "ready"
	INDEXING    IndexStatus = "indexing"
	UNAVAILABLE IndexStatus = "unavailable"
)

// omitList contains directory names to skip during indexing
var omitList = map[string]bool{
	"$RECYCLE.BIN":              true,
	"System Volume Information": true,
	"@eaDir":                    true,
}

func init() {
	indexes = make(map[string]*Index)
}

// GetNumDirs calculates the total number of directories by summing all scanner values
func (idx *Index) GetNumDirs() uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	current := idx.getNumDirsUnlocked()
	if current == 0 {
		current = idx.previousNumDirs
	}
	return current
}

// getNumDirsUnlocked calculates NumDirs without acquiring lock (assumes lock is already held)
func (idx *Index) getNumDirsUnlocked() uint64 {
	var totalDirs uint64 = 0
	for _, scanner := range idx.scanners {
		totalDirs += scanner.numDirs
	}
	return totalDirs
}

// GetNumFiles calculates the total number of files by summing all scanner values.
func (idx *Index) GetNumFiles() uint64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	current := idx.getNumFilesUnlocked()
	// If computed value is 0 (scan in progress), use previous non-zero value
	if current == 0 && idx.previousNumFiles > 0 {
		return idx.previousNumFiles
	}
	return current
}

// getNumFilesUnlocked calculates NumFiles without acquiring lock (assumes lock is already held)
func (idx *Index) getNumFilesUnlocked() uint64 {
	var totalFiles uint64 = 0
	for _, scanner := range idx.scanners {
		totalFiles += scanner.numFiles
	}
	return totalFiles
}

// GetQuickScanTime calculates the total quick scan time by summing all scanner values
func (idx *Index) GetQuickScanTime() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getQuickScanTimeUnlocked()
}

// getQuickScanTimeUnlocked calculates QuickScanTime without acquiring lock (assumes lock is already held)
func (idx *Index) getQuickScanTimeUnlocked() int {
	var total = 0
	for _, scanner := range idx.scanners {
		total += scanner.quickScanTime
	}
	return total
}

// GetFullScanTime calculates the total full scan time by summing all scanner values
func (idx *Index) GetFullScanTime() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getFullScanTimeUnlocked()
}

// getFullScanTimeUnlocked calculates FullScanTime without acquiring lock (assumes lock is already held)
func (idx *Index) getFullScanTimeUnlocked() int {
	var total = 0
	for _, scanner := range idx.scanners {
		total += scanner.fullScanTime
	}
	return total
}

// GetComplexity calculates the complexity based on full scan time and number of directories
func (idx *Index) GetComplexity() uint {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getComplexityUnlocked()
}

// getComplexityUnlocked calculates Complexity without acquiring lock (assumes lock is already held)
func (idx *Index) getComplexityUnlocked() uint {
	// If we have a persisted complexity value and scanners haven't been scanned yet, use it
	if idx.Complexity > 0 && len(idx.scanners) == 0 {
		return idx.Complexity
	}

	if len(idx.scanners) == 0 {
		// No scanners yet, but check if we have a persisted value
		if idx.Complexity > 0 {
			return idx.Complexity
		}
		return 0
	}

	allScannedAtLeastOnce := true
	for _, scanner := range idx.scanners {
		if scanner.lastScanned.IsZero() {
			allScannedAtLeastOnce = false
			break
		}
	}

	if !allScannedAtLeastOnce {
		// Scanners haven't completed first scan yet, use persisted value if available
		if idx.Complexity > 0 {
			return idx.Complexity
		}
		return 0
	}

	totalFullScanTime := idx.getFullScanTimeUnlocked()
	totalDirs := idx.getNumDirsUnlocked()
	calculatedComplexity := calculateComplexity(totalFullScanTime, totalDirs)

	// Update persisted value with calculated value
	if calculatedComplexity > 0 {
		idx.Complexity = calculatedComplexity
	}

	return calculatedComplexity
}

// GetLastIndexed returns the most recent scan time from all scanners
func (idx *Index) GetLastIndexed() time.Time {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getLastIndexedUnlocked()
}

// getLastIndexedUnlocked calculates LastIndexed without acquiring lock (assumes lock is already held)
func (idx *Index) getLastIndexedUnlocked() time.Time {
	var mostRecentScan time.Time
	for _, scanner := range idx.scanners {
		if scanner.lastScanned.After(mostRecentScan) {
			mostRecentScan = scanner.lastScanned
		}
	}
	return mostRecentScan
}

// GetStatus returns the computed status based on scanner state, or UNAVAILABLE if explicitly set
func (idx *Index) GetStatus() IndexStatus {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getStatusUnlocked()
}

// getStatusUnlocked calculates Status without acquiring lock (assumes lock is already held)
func (idx *Index) getStatusUnlocked() IndexStatus {
	if idx.Status == UNAVAILABLE {
		return UNAVAILABLE
	}

	if len(idx.scanners) == 0 {
		return idx.Status // Return current status if no scanners
	}

	allScannedAtLeastOnce := true
	anyScannerActive := false

	for _, scanner := range idx.scanners {
		if scanner.lastScanned.IsZero() {
			allScannedAtLeastOnce = false
		}
		if scanner.isScanning {
			anyScannerActive = true
		}
	}

	if anyScannerActive || idx.getActiveScannerPathUnlocked() != "" {
		return INDEXING
	} else if allScannedAtLeastOnce {
		return READY
	}

	return idx.Status
}

// InitializeIndexDB creates the shared index database for all sources.
func InitializeIndexDB() error {
	var err error
	indexDB, err = dbsql.NewIndexDB("all")
	if err != nil {
		logger.Fatalf("failed to initialize index database: %v", err)
		return err
	}

	return nil
}

// GetIndexDB returns the shared index database.
func GetIndexDB() *dbsql.IndexDB {
	return indexDB
}

// StopAllScanners stops all scanners for all indexes
// This should be called during graceful shutdown before closing the database
func StopAllScanners() {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()
	for _, idx := range indexes {
		idx.mu.Lock()
		for _, scanner := range idx.scanners {
			scanner.stop()
		}
		idx.mu.Unlock()
	}
}

// SetIndexDBForTesting sets the index database for testing purposes.
func SetIndexDBForTesting(db *dbsql.IndexDB) {
	indexDB = db
}

// SetIndexingStorage sets the persistent storage for index metadata.
func SetIndexingStorage(storage *indexingdb.Storage) {
	indexingStorage = storage
}

func Initialize(source *settings.Source, mock bool) {
	indexesMutex.Lock()
	// Use shared database - all sources are differentiated by the source column
	if indexDB == nil {
		logger.Errorf("index database not initialized, call InitializeIndexDB() first")
		indexesMutex.Unlock()
		return
	}

	newIndex := Index{
		mock:             mock,
		Source:           *source,
		db:               indexDB, // Use shared database
		scanUpdatedPaths: make(map[string]bool),
		folderSizes:      make(map[string]uint64), // In-memory folder size tracking
	}
	newIndex.ReducedIndex = ReducedIndex{
		Status:  "indexing",
		IdxName: source.Name,
		Stats: Stats{
			Complexity: 0,
		},
	}
	indexes[newIndex.Name] = &newIndex
	indexesMutex.Unlock()
	if !newIndex.Config.DisableIndexing {
		logger.Infof("initializing index: [%v]", newIndex.Name)
		// Start multi-scanner system (each scanner will do its own initial scan)
		go newIndex.setupIndexingScanners()
	} else {
		newIndex.Status = "ready"
		logger.Debug("indexing disabled for source: " + newIndex.Name)
	}
}

// Define a function to recursively index files and directories
// Returns the total size of the directory and whether it has a preview
// Size is calculated in-memory during recursive traversal to avoid expensive SQL queries
// scanner parameter is optional - if nil, will use temporary state (for API-triggered refreshes)
func (idx *Index) indexDirectory(adjustedPath string, config actionConfig, scanner *Scanner) (int64, bool, error) {
	// Normalize path to always have trailing slash
	adjustedPath = utils.AddTrailingSlashIfNotExists(adjustedPath)
	realPath := strings.TrimRight(idx.Path, "/") + adjustedPath
	// Open the directory
	dir, err := os.Open(realPath)
	if err != nil {
		// must have been deleted
		return 0, false, err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return 0, false, err
	}

	// check if excluded from indexing
	hidden := isHidden(dirInfo, idx.Path+adjustedPath)
	if idx.shouldSkip(dirInfo.IsDir(), hidden, adjustedPath, dirInfo.Name(), config) {
		return 0, false, errors.ErrNotIndexed
	}

	// adjustedPath is already normalized with trailing slash
	combinedPath := adjustedPath
	dirFileInfo, err2 := idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, config, scanner)
	if err2 != nil {
		return 0, false, err2
	}
	idx.UpdateMetadata(dirFileInfo, scanner)

	// Store the calculated directory size in the in-memory map
	// Skip for root scanner (non-recursive) - root size calculated after all child scanners complete
	if config.Recursive {
		idx.SetFolderSize(adjustedPath, uint64(dirFileInfo.Size))
	}

	return dirFileInfo.Size, dirFileInfo.HasPreview, nil
}

func (idx *Index) GetFsDirInfo(adjustedPath string) (*iteminfo.FileInfo, error) {
	realPath, isDir, err := idx.GetRealPath(adjustedPath)
	if err != nil {
		return nil, err
	}
	originalPath := realPath

	dir, err := os.Open(realPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return nil, err
	}

	if !dirInfo.IsDir() {
		realSize, _ := idx.handleFile(dirInfo, adjustedPath, realPath, false, nil) // nil scanner for FS read
		size := int64(realSize)
		fileInfo := iteminfo.FileInfo{
			Path: adjustedPath,
			ItemInfo: iteminfo.ItemInfo{
				Name:    filepath.Base(originalPath),
				Size:    size,
				ModTime: dirInfo.ModTime(),
			},
		}
		fileInfo.DetectType(realPath, false)
		setFilePreviewFlags(&fileInfo.ItemInfo, realPath)
		return &fileInfo, nil
	}
	adjustedPath = utils.AddTrailingSlashIfNotExists(adjustedPath)
	combinedPath := adjustedPath
	var response *iteminfo.FileInfo
	response, err = idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, actionConfig{
		Quick:         false,
		Recursive:     false,
		CheckViewable: true,
	}, nil) // nil scanner for FS read
	if err != nil {
		return nil, err
	}
	if !isDir {
		baseName := filepath.Base(originalPath)
		_ = idx.MakeIndexPath(realPath, false)
		found := false
		for _, item := range response.Files {
			if item.Name == baseName {
				filePath := strings.TrimSuffix(adjustedPath, "/") + "/" + item.Name
				response = &iteminfo.FileInfo{
					Path:     filePath,
					ItemInfo: item.ItemInfo,
				}
				found = true
				continue
			}
		}
		if !found {
			return nil, fmt.Errorf("file not found in directory: %s", adjustedPath)
		}

	}

	// Overlay in-memory folder sizes on the response for instant updates
	// This ensures API reads get the latest sizes even if not yet synced to DB
	if response != nil && response.Type == "directory" {
		// Update the directory's own size from in-memory map
		dirPath := utils.AddTrailingSlashIfNotExists(response.Path)
		if inMemSize := idx.GetFolderSize(dirPath); inMemSize > 0 {
			logger.Debugf("[OVERLAY_SIZE] Directory %s: in-memory=%d, current=%d", dirPath, inMemSize, response.Size)
			response.Size = int64(inMemSize)
		}

		// Update child folder sizes from in-memory map
		for i := range response.Folders {
			// Construct child path: handle root "/" specially to avoid double slashes
			var childPath string
			if response.Path == "/" {
				childPath = "/" + response.Folders[i].Name + "/"
			} else {
				childPath = utils.AddTrailingSlashIfNotExists(response.Path) + response.Folders[i].Name + "/"
			}

			if inMemSize := idx.GetFolderSize(childPath); inMemSize > 0 {
				logger.Debugf("[OVERLAY_SIZE] Child folder %s: in-memory=%d, current=%d", childPath, inMemSize, response.Folders[i].Size)
				response.Folders[i].Size = int64(inMemSize)
			} else {
				logger.Debugf("[OVERLAY_SIZE] Child folder %s: not found in memory (current=%d)", childPath, response.Folders[i].Size)
			}
		}
	}

	return response, nil

}

func (idx *Index) GetDirInfo(dirInfo *os.File, stat os.FileInfo, realPath, adjustedPath, combinedPath string, config actionConfig, scanner *Scanner) (*iteminfo.FileInfo, error) {
	combinedPath = utils.AddTrailingSlashIfNotExists(combinedPath)
	files, err := dirInfo.Readdir(-1)
	if err != nil {
		return nil, err
	}
	var totalSize int64
	fileInfos := []iteminfo.ExtendedItemInfo{}
	dirInfos := []iteminfo.ItemInfo{}
	hasPreview := false
	if !config.Recursive {
		realDirInfo, exists := idx.GetMetadataInfo(adjustedPath, true)
		if exists {
			hasPreview = realDirInfo.HasPreview
		}
	}
	for _, file := range files {
		hidden := isHidden(file, idx.Path+combinedPath)
		isDir := iteminfo.IsDirectory(file)
		baseName := file.Name()
		fullCombined := combinedPath + baseName

		// Check for symlinks if ignoreAllSymlinks is enabled (check before other skip logic)
		if idx.Config.ResolvedConditionals != nil && idx.Config.ResolvedConditionals.IgnoreAllSymlinks {
			if file.Mode()&os.ModeSymlink != 0 {
				continue
			}
		}

		if adjustedPath == "/" {
			if !idx.shouldInclude(file.Name()) {
				continue
			}
		}
		if config.CheckViewable {
			if idx.shouldSkip(isDir, hidden, fullCombined, baseName, config) && !idx.IsViewable(isDir, fullCombined) {
				continue
			}
		} else {
			if idx.shouldSkip(isDir, hidden, fullCombined, baseName, config) {
				continue
			}
		}
		itemInfo := &iteminfo.ItemInfo{
			Name:    file.Name(),
			ModTime: file.ModTime(),
			Hidden:  hidden,
		}

		if isDir {
			dirPath := combinedPath + file.Name()
			if !idx.GetLastIndexed().IsZero() && config.Recursive && idx.Config.ResolvedConditionals != nil {
				if _, exists := idx.Config.ResolvedConditionals.NeverWatchPaths[fullCombined]; exists {
					continue
				}
			}
			// skip non-indexable dirs.
			if omitList[file.Name()] {
				continue
			}
			if config.Recursive {
				// clear for garbage collection
				file = nil
				subdirSize, subdirHasPreview, err := idx.indexDirectory(dirPath, config, scanner)
				if err != nil {
					logger.Errorf("Failed to index directory %s: %v", dirPath, err)
					continue
				}
				// Use the returned values directly from recursive call
				itemInfo.Size = subdirSize
				itemInfo.HasPreview = subdirHasPreview
			} else {
				// Non-recursive: subdirectory handled by its own dedicated scanner
				// Look up current values from DB (they were calculated by that subdirectory's scanner)
				realDirInfo, exists := idx.GetMetadataInfo(dirPath, true)
				if exists {
					itemInfo.Size = realDirInfo.Size
					itemInfo.HasPreview = realDirInfo.HasPreview
				} else {
					// Not yet scanned by child scanner - use 0, will be updated when that scanner completes
					itemInfo.Size = 0
					itemInfo.HasPreview = false
				}
			}
			totalSize += itemInfo.Size
			itemInfo.Type = "directory"
			dirInfos = append(dirInfos, *itemInfo)
			if config.Recursive && config.IsRoutineScan {
				idx.incrementScannerDirs()
			}
		} else {
			realFilePath := realPath + "/" + file.Name()
			size, shouldCountSize := idx.handleFile(file, fullCombined, realFilePath, config.IsRoutineScan, scanner)
			itemInfo.DetectType(realFilePath, false)
			usedCachedPreview := false
			if !idx.Config.DisableIndexing && config.Recursive {
				simpleType := strings.Split(itemInfo.Type, "/")[0]
				if simpleType == "audio" {
					previousInfo, exists := idx.GetReducedMetadata(fullCombined, false)
					if exists && time.Time.Equal(previousInfo.ModTime, file.ModTime()) {
						// File unchanged - use cached album art info (whether true or false)
						itemInfo.HasPreview = previousInfo.HasPreview
						usedCachedPreview = true
					}
				}
			}
			if !usedCachedPreview {
				setFilePreviewFlags(itemInfo, realPath+"/"+file.Name())
			}
			itemInfo.Size = int64(size)
			if itemInfo.HasPreview && iteminfo.ShouldBubbleUpToFolderPreview(*itemInfo) {
				hasPreview = true
			}
			extItemInfo := iteminfo.ExtendedItemInfo{
				ItemInfo: *itemInfo,
			}
			fileInfos = append(fileInfos, extItemInfo)
			if shouldCountSize {
				totalSize += itemInfo.Size
			}
			if config.IsRoutineScan {
				// Only increment scanner counter, not index-level (which is calculated)
				idx.incrementScannerFiles()
			}
		}
	}
	if totalSize == 0 && idx.Config.ResolvedConditionals != nil && idx.Config.ResolvedConditionals.IgnoreAllZeroSizeFolders {
		return nil, errors.ErrNotIndexed
	}
	dirFileInfo := &iteminfo.FileInfo{
		Path:    adjustedPath,
		Files:   fileInfos,
		Folders: dirInfos,
	}
	dirFileInfo.ItemInfo = iteminfo.ItemInfo{
		Name:       filepath.Base(dirInfo.Name()),
		Type:       "directory",
		Size:       totalSize,
		ModTime:    stat.ModTime(),
		HasPreview: hasPreview,
	}
	dirFileInfo.SortItems()

	return dirFileInfo, nil
}

func (idx *Index) GetRealPath(relativePath ...string) (string, bool, error) {
	combined := append([]string{idx.Path}, relativePath...)
	joinedPath := filepath.Join(combined...)
	isDir, _ := IsDirCache.Get(joinedPath + ":isdir")
	cached, ok := RealPathCache.Get(joinedPath)
	if ok && cached != "" {
		return cached, isDir, nil
	}
	absolutePath, err := filepath.Abs(joinedPath)
	if err != nil {
		return absolutePath, false, fmt.Errorf("could not get real path: %v, %s", joinedPath, err)
	}
	realPath, isDir, err := iteminfo.ResolveSymlinks(absolutePath)
	if err == nil {
		RealPathCache.Set(joinedPath, realPath)
		IsDirCache.Set(joinedPath+":isdir", isDir)
	}
	return realPath, isDir, err
}

func (idx *Index) RefreshFileInfo(opts utils.FileOptions) error {
	// Calculate target path first (before any expensive operations)
	targetPath := opts.Path
	if !opts.IsDir {
		targetPath = idx.MakeIndexPath(filepath.Dir(targetPath), true)
	}

	// Skip refresh if we already have the size in memory (avoid redundant scans)
	existingSize := idx.GetFolderSize(targetPath)
	if existingSize > 0 {
		logger.Debugf("[REFRESH] Skipping refresh for %s, already in memory (size: %d)", targetPath, existingSize)
		return nil
	}

	// Get previous size before refresh
	previousSize := existingSize

	realPath, _, err := idx.GetRealPath(targetPath)
	if err != nil {
		logger.Errorf("[REFRESH] Failed to get real path for %s: %v", targetPath, err)
		return err
	}

	// Check if directory still exists on filesystem
	_, err = os.Stat(realPath)
	if err != nil {
		// Directory deleted - clear from in-memory map only
		// Database will be updated on next scheduled scan
		idx.SetFolderSize(targetPath, 0)
		// Update parent sizes recursively in-memory
		if previousSize > 0 {
			idx.RecursiveUpdateDirSizes(targetPath, previousSize)
		}
		return nil
	}

	// Calculate new size by scanning filesystem recursively
	newSize := idx.calculateDirectorySize(realPath, targetPath)

	// Update in-memory map with new size
	idx.SetFolderSize(targetPath, newSize)

	// Update parent directory sizes recursively in-memory if size changed
	if newSize != previousSize {
		idx.RecursiveUpdateDirSizes(targetPath, previousSize)
	}

	logger.Debugf("[REFRESH] Updated in-memory size for %s: %d (was %d)", targetPath, newSize, previousSize)
	return nil
}

// calculateDirectorySize recursively calculates the total size of a directory
// and populates the in-memory folderSizes map for all subdirectories
func (idx *Index) calculateDirectorySize(realPath string, indexPath string) uint64 {
	dir, err := os.Open(realPath)
	if err != nil {
		return 0
	}
	defer dir.Close()

	files, err := dir.Readdir(-1)
	if err != nil {
		return 0
	}

	var totalSize uint64
	for _, file := range files {
		if file.IsDir() {
			// Recursively calculate subdirectory size
			childName := file.Name()
			childRealPath := realPath + "/" + childName
			childIndexPath := indexPath + childName + "/"

			// Calculate and store child directory size
			childSize := idx.calculateDirectorySize(childRealPath, childIndexPath)
			idx.SetFolderSize(childIndexPath, childSize)
			totalSize += childSize
		} else {
			// For files, use handleFile to get accurate size (handles hardlinks, etc.)
			// This ensures consistency with scanner calculations
			childRealPath := realPath + "/" + file.Name()
			childIndexPath := indexPath + file.Name()
			size, shouldCount := idx.handleFile(file, childIndexPath, childRealPath, false, nil)
			if shouldCount {
				totalSize += size
			}
		}
	}

	return totalSize
}

// SetFolderSize sets the size for a directory in the in-memory map
// This is typically called after calculating a directory's size from its children
func (idx *Index) SetFolderSize(path string, size uint64) {
	idx.folderSizesMu.Lock()
	defer idx.folderSizesMu.Unlock()
	idx.folderSizes[path] = size
}

// GetFolderSize retrieves the size for a directory from the in-memory map
// Returns 0 if the directory size hasn't been calculated yet
func (idx *Index) GetFolderSize(path string) uint64 {
	idx.folderSizesMu.RLock()
	defer idx.folderSizesMu.RUnlock()
	size := idx.folderSizes[path]
	if size > 0 {
		logger.Debugf("[GET_FOLDER_SIZE] Path: %s → Size: %d", path, size)
	}
	return size
}

// RecursiveUpdateDirSizes updates parent directory sizes recursively up the tree
// path: the directory whose size changed
// previousSize: the previous size of the directory (used to calculate delta)
func (idx *Index) RecursiveUpdateDirSizes(path string, previousSize uint64) {
	// Get current size (should have been set before calling this)
	idx.folderSizesMu.RLock()
	currentSize, exists := idx.folderSizes[path]
	idx.folderSizesMu.RUnlock()

	if !exists {
		logger.Debugf("[FOLDER_SIZE] Path %s not found in folderSizes map", path)
		return
	}

	// Calculate delta
	var sizeDelta int64
	if currentSize >= previousSize {
		sizeDelta = int64(currentSize - previousSize)
	} else {
		sizeDelta = -int64(previousSize - currentSize)
	}

	if sizeDelta == 0 {
		return // No change, nothing to propagate
	}

	// Get parent directory
	parentDir := utils.GetParentDirectoryPath(path)
	if parentDir == "" {
		return // Reached the top
	}
	if parentDir != "/" {
		parentDir = utils.AddTrailingSlashIfNotExists(parentDir)
	}

	// Update parent size with delta
	idx.folderSizesMu.Lock()
	parentSize := idx.folderSizes[parentDir]
	if sizeDelta > 0 {
		idx.folderSizes[parentDir] = parentSize + uint64(sizeDelta)
	} else {
		// Prevent underflow
		absDelta := uint64(-sizeDelta)
		if parentSize >= absDelta {
			idx.folderSizes[parentDir] = parentSize - absDelta
		} else {
			logger.Warningf("[FOLDER_SIZE] Parent %s would underflow (%d - %d), setting to 0",
				parentDir, parentSize, absDelta)
			idx.folderSizes[parentDir] = 0
		}
	}
	previousParentSize := parentSize
	idx.folderSizesMu.Unlock()

	// Recursively update grandparents
	idx.RecursiveUpdateDirSizes(parentDir, previousParentSize)
}

// SyncFolderSizesToDB syncs in-memory folder sizes to the database after a scan completes
// Uses a single SQL operation that only updates folders where the size has changed
func (idx *Index) SyncFolderSizesToDB() error {
	idx.folderSizesMu.RLock()

	// Collect all folder sizes
	sizesToSync := make(map[string]uint64, len(idx.folderSizes))
	for path, size := range idx.folderSizes {
		sizesToSync[path] = size
	}
	idx.folderSizesMu.RUnlock()

	if len(sizesToSync) == 0 {
		return nil
	}

	// Single database operation: update only folders where size changed
	// The SQL WHERE clause filters out unchanged rows, minimizing transaction footprint
	logger.Infof("[FOLDER_SIZE_SYNC] Syncing %d folder sizes to database (will skip unchanged)", len(sizesToSync))
	rowsUpdated, err := idx.db.UpdateFolderSizesIfChanged(idx.Name, sizesToSync)
	if err != nil {
		logger.Errorf("[FOLDER_SIZE_SYNC] Failed to update folder sizes: %v", err)
		return err
	}

	if rowsUpdated > 0 {
		logger.Infof("[FOLDER_SIZE_SYNC] Updated %d folder sizes (skipped %d unchanged)",
			rowsUpdated, len(sizesToSync)-int(rowsUpdated))
	} else {
		logger.Debugf("[FOLDER_SIZE_SYNC] No folder sizes changed")
	}

	return nil
}

func isHidden(file os.FileInfo, srcPath string) bool {
	if file.Name()[0] == '.' {
		return true
	}
	if runtime.GOOS == "windows" {
		return CheckWindowsHidden(filepath.Join(srcPath, file.Name()))
	}
	// Default behavior for non-Windows systems
	return false
}

// setFilePreviewFlags determines if a file should have a preview based on its type
func setFilePreviewFlags(fileInfo *iteminfo.ItemInfo, realPath string) {
	simpleType := strings.Split(fileInfo.Type, "/")[0]
	switch fileInfo.Type {
	case "image/heic", "image/heif":
		fileInfo.HasPreview = settings.CanConvertImage("heic")
		return
	}
	switch simpleType {
	case "image":
		fileInfo.HasPreview = true
		return
	case "video":
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileInfo.Name)), ".")
		fileInfo.HasPreview = settings.CanConvertVideo(ext)
		return
	case "audio":
		ext := strings.ToLower(filepath.Ext(fileInfo.Name))
		hasArt := iteminfo.HasAlbumArt(realPath, ext)
		fileInfo.HasPreview = hasArt
		return
	}
	// Check for office docs and PDFs
	if settings.Config.Integrations.OnlyOffice.Secret != "" && iteminfo.IsOnlyOffice(fileInfo.Name) {
		fileInfo.HasPreview = true
		return
	}
	if iteminfo.HasDocConvertableExtension(fileInfo.Name, fileInfo.Type) {
		fileInfo.HasPreview = true
		return
	}
}

// IsViewable checks if a path has viewable:true (allows FS access without indexing)
func (idx *Index) IsViewable(isDir bool, adjustedPath string) bool {
	rules := idx.Config.ResolvedConditionals

	baseName := filepath.Base(strings.TrimSuffix(adjustedPath, "/"))

	if isDir {
		if rule, exists := rules.FolderNames[baseName]; exists && rule.Viewable {
			return true
		}
		for _, rule := range rules.FolderPaths {
			if strings.HasPrefix(adjustedPath, rule.FolderPath) && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FolderEndsWith {
			if strings.HasSuffix(baseName, rule.FolderEndsWith) && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FolderStartsWith {
			if strings.HasPrefix(baseName, rule.FolderStartsWith) && rule.Viewable {
				return true
			}
		}
	} else {
		if rule, exists := rules.FileNames[baseName]; exists && rule.Viewable {
			return true
		}
		for _, rule := range rules.FilePaths {
			if strings.HasPrefix(adjustedPath, rule.FilePath) && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FileEndsWith {
			if strings.HasSuffix(baseName, rule.FileEndsWith) && rule.Viewable {
				return true
			}
		}
		for _, rule := range rules.FileStartsWith {
			if strings.HasPrefix(baseName, rule.FileStartsWith) && rule.Viewable {
				return true
			}
		}
	}
	return false
}

func (idx *Index) shouldSkip(isDir bool, isHidden bool, fullCombined, baseName string, config actionConfig) bool {
	rules := idx.Config.ResolvedConditionals
	if rules == nil {
		rules = &settings.ResolvedConditionalsConfig{}
	}
	if fullCombined == "/" {
		return false
	}
	if idx.Config.DisableIndexing {
		return !config.CheckViewable
	}

	if isDir && config.IsRoutineScan {
		_, ok := rules.NeverWatchPaths[fullCombined]
		if ok {
			return true
		}
	}

	if isDir {
		if rule, ok := rules.FolderNames[baseName]; ok {
			if _, ok := rules.FolderPaths[fullCombined]; !ok {
				rules.FolderPaths[fullCombined] = rule
			}
			return true
		}
		if _, ok := rules.FolderPaths[fullCombined]; ok {
			return true
		}
		for path := range rules.FolderPaths {
			if strings.HasPrefix(fullCombined, path) {
				return true
			}
		}

		// Check FolderEndsWith (suffix match on base name) - use original slice
		if len(rules.FolderEndsWith) > 0 {
			for _, rule := range rules.FolderEndsWith {
				if hasSuffix := strings.HasSuffix(baseName, rule.FolderEndsWith); hasSuffix {
					return true
				}
			}
		}
		for _, rule := range rules.FolderStartsWith {
			if hasPrefix := strings.HasPrefix(baseName, rule.FolderStartsWith); hasPrefix {
				return true
			}
		}
	} else {
		if _, ok := rules.FileNames[baseName]; ok {
			return true
		}
		if _, ok := rules.FilePaths[fullCombined]; ok {
			return true
		}
		for path := range rules.FilePaths {
			if strings.HasPrefix(fullCombined, path) {
				return true
			}
		}

		for _, rule := range rules.FileEndsWith {
			if hasSuffix := strings.HasSuffix(baseName, rule.FileEndsWith); hasSuffix {
				return true
			}
		}
		for _, rule := range rules.FileStartsWith {
			if hasPrefix := strings.HasPrefix(baseName, rule.FileStartsWith); hasPrefix {
				return true
			}
		}

	}

	if idx.Config.ResolvedConditionals != nil && idx.Config.ResolvedConditionals.IgnoreAllHidden && isHidden {
		return true
	}

	return false
}

type DiskUsage struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

func (idx *Index) SetUsage(totalBytes uint64) {
	if settings.Config.Frontend.DisableUsedPercentage {
		return
	}
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.DiskTotal = totalBytes
}

func (idx *Index) SetStatus(status IndexStatus) error {
	idx.mu.Lock()
	idx.Status = status
	idx.mu.Unlock()
	return idx.SendSourceUpdateEvent()
}

// getActiveScannerPath returns the path of the currently active scanner, or empty string if none
// Assumes mutex is NOT held - will acquire its own lock
func (idx *Index) getActiveScannerPath() string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getActiveScannerPathUnlocked()
}

// getActiveScannerPathUnlocked returns the path of the currently active scanner, or empty string if none
func (idx *Index) getActiveScannerPathUnlocked() string {
	for path, scanner := range idx.scanners {
		if scanner.isScanning {
			return path
		}
	}
	return ""
}

// getRunningScannerCount returns the number of scanners currently running
// Assumes mutex is NOT held - will acquire its own lock
func (idx *Index) getRunningScannerCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getRunningScannerCountUnlocked()
}

// getRunningScannerCountUnlocked returns the number of scanners currently running
// Assumes mutex IS already held (RLock or Lock)
func (idx *Index) getRunningScannerCountUnlocked() int {
	count := 0
	for _, scanner := range idx.scanners {
		if scanner.isScanning {
			count++
		}
	}
	return count
}

// input should be non-index path.
// isDir indicates whether the path is a directory (true) or file (false).
// Directories will have a trailing slash added, files will not.
func (idx *Index) MakeIndexPath(path string, isDir bool) string {
	if path == "." || strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, ".")
	}
	path = strings.TrimPrefix(path, idx.Path)
	path = idx.MakeIndexPathPlatform(path)
	if isDir {
		path = utils.AddTrailingSlashIfNotExists(path)
	}
	return path
}

// MakeAbsolutePath converts a relative index path to an absolute path by combining
// the source path with the relative path. This ensures paths are unique across sources.
func (idx *Index) MakeAbsolutePath(indexPath string) string {
	return idx.Path + indexPath
}

func (idx *Index) shouldInclude(baseName string) bool {
	rules := idx.Config.ResolvedConditionals
	if rules == nil {
		rules = &settings.ResolvedConditionalsConfig{}
	}
	hasRules := false
	if len(rules.IncludeRootItems) > 0 {
		hasRules = true
		if _, exists := rules.IncludeRootItems["/"+baseName]; exists {
			return true
		}
	}
	if !hasRules {
		return true
	}
	return false
}

// Save persists the index and scanner information to the database
func (idx *Index) Save() error {
	if indexingStorage == nil {
		return nil // No storage available, skip persistence
	}

	idx.mu.RLock()
	// Collect scanner information
	scanners := make(map[string]*indexingdb.PersistedScannerInfo)
	for path, scanner := range idx.scanners {
		scanners[path] = &indexingdb.PersistedScannerInfo{
			Path:            path,
			Complexity:      scanner.complexity,
			CurrentSchedule: scanner.currentSchedule,
			QuickScanTime:   scanner.quickScanTime,
			FullScanTime:    scanner.fullScanTime,
			NumDirs:         scanner.numDirs,
			NumFiles:        scanner.numFiles,
			LastScanned:     scanner.lastScanned,
		}
	}

	// Get current index stats
	complexity := idx.getComplexityUnlocked()
	numDirs := idx.getNumDirsUnlocked()
	numFiles := idx.getNumFilesUnlocked()
	idx.mu.RUnlock()

	// Create IndexInfo for persistence
	info := &indexingdb.IndexInfo{
		Path:       idx.Path, // Use real filesystem path as key
		Source:     idx.Name,
		Complexity: complexity,
		NumDirs:    numDirs,
		NumFiles:   numFiles,
		Scanners:   scanners,
	}

	return indexingStorage.Save(info)
}

// Load restores index and scanner information from the database
func (idx *Index) Load() error {
	if indexingStorage == nil {
		return nil // No storage available, skip loading
	}

	info, err := indexingStorage.GetByPath(idx.Path)
	if err != nil {
		if err == errors.ErrNotExist {
			// No persisted data exists, this is fine for new indexes
			return nil
		}
		return err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Restore index-level stats
	idx.previousNumDirs = info.NumDirs
	idx.previousNumFiles = info.NumFiles
	idx.Complexity = info.Complexity
	idx.NumDirs = info.NumDirs
	idx.NumFiles = info.NumFiles

	// Restore scanner information (will be applied when scanners are created)
	// Store in a temporary map that setupMultiScanner can use
	if idx.scanners == nil {
		idx.scanners = make(map[string]*Scanner)
	}

	// Note: Scanners will be created by setupMultiScanner, but we'll restore
	// their stats after creation. Store the persisted scanner info for later use.
	// We'll handle this in setupMultiScanner after scanners are created.

	return nil
}

// Flush persists all index information to the database
func (idx *Index) Flush() error {
	return idx.Save()
}
