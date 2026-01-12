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

// getDiskUsage returns the actual disk space used by a file in bytes
// This is a simplified helper for shallow calculations and API calls
// For scanning with hardlink detection, use handleFile instead
// If useLogicalSize is true, returns the logical file size instead
func getDiskUsage(fileInfo os.FileInfo, realPath string, useLogicalSize bool) int64 {
	// If useLogicalSize is true, return logical size (no block alignment)
	if useLogicalSize {
		if fileInfo.IsDir() {
			return 0 // Directories have no logical size
		}
		return fileInfo.Size()
	}

	// Disk usage mode (du-like behavior)
	// For directories, use minimum 4KB
	if fileInfo.IsDir() {
		return 4096
	}

	// For files, round up to nearest 4KB block for non-zero files
	// Note: handleFile in unix.go/windows.go provides more accurate disk usage during scanning
	size := fileInfo.Size()
	if size == 0 {
		return 0
	}
	// Round up to nearest 4KB
	blocks := (size + 4095) / 4096
	return blocks * 4096
}

// getFileSizeForListing returns the file size for directory listing display
// This is the logical size, not disk usage
func getFileSizeForListing(fileInfo os.FileInfo) int64 {
	return fileInfo.Size()
}

// Options holds all configuration options for indexing and filesystem operations
type Options struct {
	// Indexing operation options
	Quick         bool // whether to perform a quick scan (skip unchanged directories)
	Recursive     bool // whether to recursively index subdirectories
	CheckViewable bool // whether to check if the path has viewable:true (for API access checks)
	IsRoutineScan bool // whether this is a routine/scheduled scan (vs initial indexing)

	// Filesystem info retrieval options
	SkipIndexChecks   bool // Skip shouldSkip checks (for viewable-only paths)
	SkipExtendedAttrs bool // Skip hasPreview and other extended attributes
	UseInMemorySizes  bool // Use in-memory folder size cache vs filesystem calculation
	FollowSymlinks    bool // Whether to follow symlinks or return symlink info
	ShowHidden        bool // Whether to include hidden files/directories
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
	folderSizes         map[string]uint64   // path -> size (in-memory only, calculated from children)
	folderSizesUnsynced map[string]struct{} // Tracks which folder sizes have changed since last DB sync
	folderSizesMu       sync.RWMutex        // Dedicated RWMutex allows concurrent reads, serializes writes
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
	journalMode := "OFF"
	if settings.Config.Server.IndexSqlConfig.WalMode {
		journalMode = "WAL"
	}
	batchSize := settings.Config.Server.IndexSqlConfig.BatchSize
	cacheSizeMB := settings.Config.Server.IndexSqlConfig.CacheSizeMB
	disableReuse := settings.Config.Server.IndexSqlConfig.DisableReuse
	indexDB, err = dbsql.NewIndexDB("all", journalMode, batchSize, cacheSizeMB, disableReuse)
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
		mock:                mock,
		Source:              *source,
		db:                  indexDB, // Use shared database
		scanUpdatedPaths:    make(map[string]bool),
		folderSizes:         make(map[string]uint64),   // In-memory folder size tracking
		folderSizesUnsynced: make(map[string]struct{}), // Track changed folders
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
func (idx *Index) indexDirectory(adjustedPath string, opts Options, scanner *Scanner) (int64, bool, error) {
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
	if idx.shouldSkip(dirInfo.IsDir(), hidden, adjustedPath, dirInfo.Name(), opts) {
		return 0, false, errors.ErrNotIndexed
	}

	// adjustedPath is already normalized with trailing slash
	combinedPath := adjustedPath
	dirFileInfo, err2 := idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, opts, scanner)
	if err2 != nil {
		return 0, false, err2
	}
	idx.UpdateMetadata(dirFileInfo, scanner)

	// Store the calculated directory size in the in-memory map
	// Skip for root scanner (non-recursive) - root size calculated after all child scanners complete
	if opts.Recursive {
		idx.SetFolderSize(adjustedPath, uint64(dirFileInfo.Size))
	}

	return dirFileInfo.Size, dirFileInfo.HasPreview, nil
}

// GetFsInfoCore is the consolidated implementation for both GetFsInfo and GetFsInfoViewableOnly
// If dir and dirInfo are provided, they will be used instead of opening/statting again
func (idx *Index) GetFsInfoCore(adjustedPath string, opts Options, dir *os.File, dirInfo os.FileInfo) (*iteminfo.FileInfo, error) {
	// Handle symlinks if not following them
	if !opts.FollowSymlinks {
		realPath := filepath.Join(idx.Path, adjustedPath)
		symlinkInfo, err := os.Lstat(realPath)
		if err != nil {
			return nil, err
		}

		// Check index if not skipping index checks
		if !opts.SkipIndexChecks {
			hidden := isHidden(symlinkInfo, idx.Path+adjustedPath)
			if idx.shouldSkip(symlinkInfo.IsDir(), hidden, adjustedPath, symlinkInfo.Name(), Options{
				Quick:         false,
				Recursive:     false,
				CheckViewable: opts.CheckViewable,
			}) {
				return nil, errors.ErrNotIndexed
			}
		}

		// If it's a symlink, return info about the symlink itself
		if symlinkInfo.Mode()&os.ModeSymlink != 0 {
			realSize, _ := idx.handleFile(symlinkInfo, adjustedPath, realPath, false, nil)
			return &iteminfo.FileInfo{
				Path: adjustedPath,
				ItemInfo: iteminfo.ItemInfo{
					Name:    filepath.Base(strings.TrimSuffix(adjustedPath, "/")),
					Size:    int64(realSize),
					ModTime: symlinkInfo.ModTime(),
					Type:    "symlink",
				},
			}, nil
		}
	}

	realPath, isDir, err := idx.GetRealPath(adjustedPath)
	if err != nil {
		return nil, err
	}
	originalPath := realPath

	// Open directory if not provided
	if dir == nil {
		dir, err = os.Open(realPath)
		if err != nil {
			return nil, err
		}
		defer dir.Close()
	}

	// Get dirInfo if not provided
	if dirInfo == nil {
		dirInfo, err = dir.Stat()
		if err != nil {
			return nil, err
		}
	}

	// Handle file case
	if !dirInfo.IsDir() {
		// Check index if not skipping index checks
		if !opts.SkipIndexChecks {
			hidden := isHidden(dirInfo, idx.Path+adjustedPath)
			if idx.shouldSkip(dirInfo.IsDir(), hidden, adjustedPath, dirInfo.Name(), Options{
				Quick:         false,
				Recursive:     false,
				CheckViewable: opts.CheckViewable,
			}) {
				return nil, errors.ErrNotIndexed
			}
		}

		realSize, _ := idx.handleFile(dirInfo, adjustedPath, realPath, false, nil)
		fileInfo := &iteminfo.FileInfo{
			Path: adjustedPath,
			ItemInfo: iteminfo.ItemInfo{
				Name:    filepath.Base(originalPath),
				Size:    int64(realSize),
				ModTime: dirInfo.ModTime(),
			},
		}
		fileInfo.DetectType(realPath, false)
		if !opts.SkipExtendedAttrs {
			setFilePreviewFlags(&fileInfo.ItemInfo, realPath)
		}
		return fileInfo, nil
	}

	// Handle directory case
	adjustedPath = utils.AddTrailingSlashIfNotExists(adjustedPath)
	response, err := idx.GetDirInfoCore(dir, dirInfo, realPath, adjustedPath, adjustedPath, opts, nil)
	if err != nil {
		return nil, err
	}

	// Handle file-in-directory case
	if !isDir {
		baseName := filepath.Base(originalPath)
		if !opts.SkipIndexChecks {
			_ = idx.MakeIndexPath(realPath, false)
		}
		for _, item := range response.Files {
			if item.Name == baseName {
				return &iteminfo.FileInfo{
					Path:     strings.TrimSuffix(adjustedPath, "/") + "/" + item.Name,
					ItemInfo: item.ItemInfo,
				}, nil
			}
		}
		return nil, fmt.Errorf("file not found in directory: %s", adjustedPath)
	}

	return response, nil
}

// GetFsInfo returns filesystem information with index checks and extended attributes
func (idx *Index) GetFsInfo(adjustedPath string, followSymlinks bool, showHidden bool) (*iteminfo.FileInfo, error) {
	return idx.GetFsInfoCore(adjustedPath, Options{
		Quick:             false,
		Recursive:         false,
		CheckViewable:     true,
		IsRoutineScan:     false,
		SkipIndexChecks:   false,
		SkipExtendedAttrs: false,
		UseInMemorySizes:  true,
		FollowSymlinks:    followSymlinks,
		ShowHidden:        showHidden,
	}, nil, nil)
}

// GetFsInfoViewableOnly returns filesystem information for viewable-only paths (not indexed)
func (idx *Index) GetFsInfoViewableOnly(adjustedPath string, followSymlinks bool, showHidden bool) (*iteminfo.FileInfo, error) {
	return idx.GetFsInfoCore(adjustedPath, Options{
		Quick:             false,
		Recursive:         false,
		CheckViewable:     false,
		IsRoutineScan:     false,
		SkipIndexChecks:   true,
		SkipExtendedAttrs: true,
		UseInMemorySizes:  false,
		FollowSymlinks:    followSymlinks,
		ShowHidden:        showHidden,
	}, nil, nil)
}

// fetchExtendedAttributes fetches hasPreview for the current directory and batch fetches for subdirectories
// Returns the current directory's hasPreview and a map of subdirectory paths to their hasPreview values
func (idx *Index) fetchExtendedAttributes(adjustedPath, combinedPath string, files []os.FileInfo, opts Options) (bool, map[string]bool) {
	hasPreview := false
	subdirHasPreviewMap := make(map[string]bool)

	if opts.SkipExtendedAttrs || opts.Recursive {
		return hasPreview, subdirHasPreviewMap
	}

	// Fetch hasPreview for current directory
	realDirInfo, exists := idx.GetMetadataInfo(adjustedPath, true, true)
	if exists {
		hasPreview = realDirInfo.HasPreview
	}

	// Batch fetch hasPreview for all subdirectories
	var subdirPaths []string
	for _, file := range files {
		if !iteminfo.IsDirectory(file) {
			continue
		}
		baseName := file.Name()
		if adjustedPath == "/" {
			if !idx.shouldInclude(baseName) {
				continue
			}
		}
		if omitList[baseName] {
			continue
		}
		dirPath := combinedPath + baseName
		childIndexPath := utils.AddTrailingSlashIfNotExists(dirPath)
		subdirPaths = append(subdirPaths, childIndexPath)
	}

	if len(subdirPaths) > 0 {
		var err error
		subdirHasPreviewMap, err = idx.db.GetHasPreviewBatch(idx.Name, subdirPaths)
		if err != nil {
			subdirHasPreviewMap = make(map[string]bool)
		}
	}

	return hasPreview, subdirHasPreviewMap
}

// shouldProcessItem determines if an item should be processed based on skip rules, viewable rules, and symlink settings
func (idx *Index) shouldProcessItem(file os.FileInfo, adjustedPath, combinedPath, baseName string, isDir bool, opts Options) bool {
	fullCombined := combinedPath + baseName

	// Check for symlinks if ignoreAllSymlinks is enabled
	if idx.Config.ResolvedConditionals != nil && idx.Config.ResolvedConditionals.IgnoreAllSymlinks {
		if file.Mode()&os.ModeSymlink != 0 {
			return false
		}
	}

	// Check ShowHidden option - filter out hidden files if ShowHidden is false
	if !opts.ShowHidden {
		hidden := isHidden(file, idx.Path+combinedPath)
		if hidden {
			return false
		}
	}

	// Check root include rules
	if adjustedPath == "/" {
		if !idx.shouldInclude(file.Name()) {
			return false
		}
	}

	// Index/viewable checking logic
	if opts.SkipIndexChecks {
		// Viewable-only: only check IsViewable
		return idx.IsViewable(isDir, fullCombined)
	}

	// Indexed: use shouldSkip with optional viewable check
	hidden := isHidden(file, idx.Path+combinedPath)
	if opts.CheckViewable {
		return !idx.shouldSkip(isDir, hidden, fullCombined, baseName, opts) || idx.IsViewable(isDir, fullCombined)
	}
	return !idx.shouldSkip(isDir, hidden, fullCombined, baseName, opts)
}

// processDirectoryItem processes a directory item and returns the itemInfo, size, and whether it should be counted
func (idx *Index) processDirectoryItem(file os.FileInfo, combinedPath, realPath, fullCombined string, subdirHasPreviewMap map[string]bool, opts Options, scanner *Scanner) (*iteminfo.ItemInfo, int64, bool) {
	dirPath := combinedPath + file.Name()

	// Check NeverWatchPaths for recursive scans
	if !idx.GetLastIndexed().IsZero() && opts.Recursive && idx.Config.ResolvedConditionals != nil {
		if _, exists := idx.Config.ResolvedConditionals.NeverWatchPaths[fullCombined]; exists {
			return nil, 0, false
		}
	}

	// Skip non-indexable dirs
	if omitList[file.Name()] {
		return nil, 0, false
	}

	itemInfo := &iteminfo.ItemInfo{
		Name:    file.Name(),
		ModTime: file.ModTime(),
		Hidden:  isHidden(file, idx.Path+combinedPath),
		Type:    "directory",
	}

	if opts.Recursive {
		// Recursive: index the subdirectory
		subdirSize, subdirHasPreview, err := idx.indexDirectory(dirPath, opts, scanner)
		if err != nil {
			logger.Errorf("Failed to index directory %s: %v", dirPath, err)
			return nil, 0, false
		}
		// Apply minimum 4KB for directories only in disk usage mode
		if !idx.Config.UseLogicalSize && subdirSize < 4096 {
			subdirSize = 4096
		}
		itemInfo.Size = subdirSize
		itemInfo.HasPreview = subdirHasPreview
		return itemInfo, itemInfo.Size, true
	}

	// Non-recursive: get folder size and hasPreview
	if opts.UseInMemorySizes {
		// Use in-memory folder size (fast) with config-aware formatting
		childIndexPath := utils.AddTrailingSlashIfNotExists(dirPath)
		itemInfo.Size = idx.GetFolderSizeForDisplay(childIndexPath)

		// Use batched hasPreview map if available
		if !opts.SkipExtendedAttrs && subdirHasPreviewMap != nil {
			if hasPreviewValue, exists := subdirHasPreviewMap[childIndexPath]; exists {
				itemInfo.HasPreview = hasPreviewValue
			} else {
				itemInfo.HasPreview = false
			}
		} else {
			itemInfo.HasPreview = false
		}
	} else {
		// Calculate size from filesystem (shallow, just immediate children)
		childRealPath := realPath + "/" + file.Name()
		childDir, err := os.Open(childRealPath)
		if err == nil {
			childFiles, err := childDir.Readdir(-1)
			childDir.Close()
			if err == nil {
				var dirSize int64
				for _, childFile := range childFiles {
					if childFile.IsDir() {
						// Count subdirectories - use helper for consistent logic
						if !idx.Config.UseLogicalSize {
							dirSize += 4096 // Disk usage: minimum 4KB each
						}
						// Logical mode: subdirs contribute 0
					} else {
						// Use helper function for file size calculation
						childFilePath := childRealPath + "/" + childFile.Name()
						fileSize := idx.getFileSizeForDisplay(childFile, childFilePath)
						dirSize += fileSize
					}
				}
				// Apply minimum 4KB for directories only in disk usage mode
				if !idx.Config.UseLogicalSize && dirSize < 4096 {
					dirSize = 4096
				}
				itemInfo.Size = dirSize
			} else {
				// Error reading directory - use appropriate default
				itemInfo.Size = idx.GetFolderSizeForDisplay("")
			}
		} else {
			// Error opening directory - use appropriate default
			itemInfo.Size = idx.GetFolderSizeForDisplay("")
		}
		itemInfo.HasPreview = false
	}

	return itemInfo, itemInfo.Size, true
}

// processFileItem processes a file item and returns the itemInfo, size, shouldCount, and whether it bubbles up hasPreview
func (idx *Index) processFileItem(file os.FileInfo, realPath, combinedPath, fullCombined string, opts Options, scanner *Scanner) (*iteminfo.ItemInfo, int64, bool, bool) {
	realFilePath := realPath + "/" + file.Name()
	itemInfo := &iteminfo.ItemInfo{
		Name:    file.Name(),
		ModTime: file.ModTime(),
		Hidden:  isHidden(file, idx.Path+combinedPath),
	}
	itemInfo.DetectType(realFilePath, false)

	// For API calls (non-recursive, no scanner), use appropriate size calculation
	// For scanning (recursive or with scanner), use handleFile for hardlink detection
	var size uint64
	var shouldCountSize bool
	if !opts.Recursive && scanner == nil {
		// API call: use helper function for config-aware size calculation
		size = uint64(idx.getFileSizeForDisplay(file, realFilePath))
		shouldCountSize = true
	} else {
		// Scanning: use handleFile for accurate size and hardlink detection
		size, shouldCountSize = idx.handleFile(file, fullCombined, realFilePath, opts.IsRoutineScan, scanner)
	}

	// Extended attributes for files
	bubblesUpHasPreview := false
	if !opts.SkipExtendedAttrs {
		usedCachedPreview := false
		if !idx.Config.DisableIndexing && opts.Recursive {
			simpleType := strings.Split(itemInfo.Type, "/")[0]
			if simpleType == "audio" {
				previousInfo, exists := idx.GetReducedMetadata(fullCombined, false)
				if exists && time.Time.Equal(previousInfo.ModTime, file.ModTime()) {
					itemInfo.HasPreview = previousInfo.HasPreview
					usedCachedPreview = true
				}
			}
		}
		if !usedCachedPreview {
			setFilePreviewFlags(itemInfo, realFilePath)
		}
		if itemInfo.HasPreview && iteminfo.ShouldBubbleUpToFolderPreview(*itemInfo) {
			bubblesUpHasPreview = true
		}
	} else {
		itemInfo.HasPreview = false
	}

	itemInfo.Size = int64(size)
	return itemInfo, itemInfo.Size, shouldCountSize, bubblesUpHasPreview
}

// getDirectoryName extracts the directory name from realPath, with fallback to adjustedPath
func getDirectoryName(realPath, adjustedPath string) string {
	dirName := filepath.Base(realPath)
	if dirName == "." || dirName == "" {
		// Fallback: use adjustedPath if realPath gives us "."
		trimmed := strings.TrimSuffix(adjustedPath, "/")
		if trimmed == "" || trimmed == "/" {
			dirName = "/"
		} else {
			dirName = filepath.Base(trimmed)
		}
	}
	return dirName
}

// GetDirInfoCore is the consolidated implementation for both GetDirInfo and GetDirInfoViewableOnly
func (idx *Index) GetDirInfoCore(dirInfo *os.File, stat os.FileInfo, realPath, adjustedPath, combinedPath string, opts Options, scanner *Scanner) (*iteminfo.FileInfo, error) {
	combinedPath = utils.AddTrailingSlashIfNotExists(combinedPath)

	// Only log for API calls, not during scanning
	isAPICall := scanner == nil
	if isAPICall {
		logger.Debugf("[GET_DIR_INFO] GetDirInfoCore: Starting for path=%s, combinedPath=%s, realPath=%s", adjustedPath, combinedPath, realPath)
	}
	files, err := dirInfo.Readdir(-1)
	if err != nil {
		if isAPICall {
			logger.Errorf("[GET_DIR_INFO] GetDirInfoCore: Readdir failed for path=%s, error=%v", adjustedPath, err)
		}
		return nil, err
	}
	if isAPICall {
		logger.Debugf("[GET_DIR_INFO] GetDirInfoCore: Read %d files from filesystem for path=%s", len(files), adjustedPath)
	}

	// Fetch extended attributes (hasPreview for current dir and subdirs)
	hasPreview, subdirHasPreviewMap := idx.fetchExtendedAttributes(adjustedPath, combinedPath, files, opts)

	var totalSize int64
	fileInfos := []iteminfo.ExtendedItemInfo{}
	dirInfos := []iteminfo.ItemInfo{}
	processedCount := 0
	skippedCount := 0

	for _, file := range files {
		baseName := file.Name()
		isDir := iteminfo.IsDirectory(file)
		fullCombined := combinedPath + baseName

		// Check if item should be processed
		if !idx.shouldProcessItem(file, adjustedPath, combinedPath, baseName, isDir, opts) {
			skippedCount++
			if isAPICall {
				logger.Debugf("[GET_DIR_INFO] GetDirInfoCore: Skipped item name=%s, isDir=%v, path=%s", baseName, isDir, fullCombined)
			}
			continue
		}
		processedCount++

		if isDir {
			itemInfo, size, shouldCount := idx.processDirectoryItem(file, combinedPath, realPath, fullCombined, subdirHasPreviewMap, opts, scanner)
			if itemInfo == nil {
				continue
			}
			if shouldCount {
				totalSize += size
			}
			dirInfos = append(dirInfos, *itemInfo)
			if opts.Recursive && opts.IsRoutineScan {
				idx.incrementScannerDirs()
			}
		} else {
			itemInfo, size, shouldCount, bubblesUp := idx.processFileItem(file, realPath, combinedPath, fullCombined, opts, scanner)
			if shouldCount {
				totalSize += size
			}
			if bubblesUp {
				hasPreview = true
			}
			fileInfos = append(fileInfos, iteminfo.ExtendedItemInfo{ItemInfo: *itemInfo})
			if opts.IsRoutineScan {
				idx.incrementScannerFiles()
			}
		}
	}

	// Check ignoreZeroSizeFolders: works consistently regardless of size calculation mode
	// In logical mode: folders with no files will have totalSize=0
	// In disk usage mode: folders with no files will have totalSize>0 from 4KB minimums,
	//                     but we check if there are no actual files/folders processed
	if idx.Config.ResolvedConditionals != nil && idx.Config.ResolvedConditionals.IgnoreAllZeroSizeFolders && combinedPath != "/" {
		// If using logical size, check totalSize==0
		// If using disk usage, check if we have no files AND no folders (truly empty)
		isEmpty := false
		if idx.Config.UseLogicalSize {
			isEmpty = (totalSize == 0)
		} else {
			// In disk usage mode, check if directory has no actual content
			// (processedCount will be 0 if truly empty)
			isEmpty = (len(fileInfos) == 0 && len(dirInfos) == 0)
		}

		if isEmpty {
			return nil, errors.ErrNotIndexed
		}
	}

	dirFileInfo := &iteminfo.FileInfo{
		Path:    adjustedPath,
		Files:   fileInfos,
		Folders: dirInfos,
	}
	dirFileInfo.ItemInfo = iteminfo.ItemInfo{
		Name:       getDirectoryName(realPath, adjustedPath),
		Type:       "directory",
		Size:       totalSize,
		ModTime:    stat.ModTime(),
		HasPreview: hasPreview,
	}
	dirFileInfo.SortItems()

	// Only log for API calls, not during scanning
	if scanner == nil {
		logger.Debugf("[GET_DIR_INFO] GetDirInfoCore: Completed for path=%s - processed=%d, skipped=%d, files=%d, folders=%d, totalSize=%d",
			adjustedPath, processedCount, skippedCount, len(fileInfos), len(dirInfos), totalSize)
	}

	return dirFileInfo, nil
}

// GetDirInfo returns directory information with index checks and extended attributes
func (idx *Index) GetDirInfo(dirInfo *os.File, stat os.FileInfo, realPath, adjustedPath, combinedPath string, opts Options, scanner *Scanner) (*iteminfo.FileInfo, error) {
	// Ensure filesystem options are set correctly for indexed paths
	opts.SkipIndexChecks = false
	opts.SkipExtendedAttrs = false
	opts.UseInMemorySizes = true
	return idx.GetDirInfoCore(dirInfo, stat, realPath, adjustedPath, combinedPath, opts, scanner)
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
	return idx.RefreshFileInfoWithHandle(opts, nil, nil)
}

// RefreshFileInfoWithHandle is the same as RefreshFileInfo but accepts an optional directory handle
// to avoid reopening the directory. If dir is nil, it will open the directory itself.
func (idx *Index) RefreshFileInfoWithHandle(opts utils.FileOptions, dir *os.File, dirInfo os.FileInfo) error {
	// Calculate target path
	targetPath := opts.Path
	if !opts.IsDir {
		targetPath = idx.MakeIndexPath(filepath.Dir(targetPath), true)
	}

	// Get real path and check if directory exists
	realPath, _, err := idx.GetRealPath(targetPath)
	if err != nil {
		return err
	}

	// Get dirInfo if not provided
	if dirInfo == nil {
		if dir != nil {
			dirInfo, err = dir.Stat()
			if err != nil {
				// Directory deleted - clear from in-memory map and update parents
				idx.folderSizesMu.Lock()
				previousSize, exists := idx.folderSizes[targetPath]
				delete(idx.folderSizes, targetPath)
				delete(idx.folderSizesUnsynced, targetPath)
				idx.folderSizesMu.Unlock()

				if exists && previousSize > 0 {
					idx.updateFolderSizeAndParents(targetPath, 0, previousSize, false) // updateTarget=false, only update parents
				}
				return nil
			}
		} else {
			dirInfo, err = os.Stat(realPath)
			if err != nil {
				// Directory deleted - clear from in-memory map and update parents
				idx.folderSizesMu.Lock()
				previousSize, exists := idx.folderSizes[targetPath]
				delete(idx.folderSizes, targetPath)
				delete(idx.folderSizesUnsynced, targetPath)
				idx.folderSizesMu.Unlock()

				if exists && previousSize > 0 {
					idx.updateFolderSizeAndParents(targetPath, 0, previousSize, false) // updateTarget=false, only update parents
				}
				return nil
			}
		}
	}

	// Check if excluded from indexing
	hidden := isHidden(dirInfo, idx.Path+targetPath)
	if idx.shouldSkip(dirInfo.IsDir(), hidden, targetPath, dirInfo.Name(), Options{}) {
		return errors.ErrNotIndexed
	}

	// Get previous size before shallow calculation
	previousSize, _ := idx.GetFolderSize(targetPath)

	newSize := idx.calculateDirectorySize(realPath, targetPath, true, dir) // shallow=true for API calls

	// Update this directory and propagate to parents if changed
	if newSize != previousSize {
		idx.updateFolderSizeAndParents(targetPath, newSize, previousSize, true) // updateTarget=true for API calls
	}

	return nil
}

// calculateDirectorySize calculates directory size with optional shallow or recursive mode
// If dir is provided, it will be used instead of opening a new one
func (idx *Index) calculateDirectorySize(realPath string, indexPath string, shallow bool, dir *os.File) uint64 {
	var err error
	if dir == nil {
		dir, err = os.Open(realPath)
		if err != nil {
			return 0
		}
		defer dir.Close()
	}

	files, err := dir.Readdir(-1)
	if err != nil {
		return 0
	}

	var totalSize uint64
	for _, file := range files {
		if file.IsDir() {
			childName := file.Name()
			childIndexPath := indexPath + childName + "/"

			if shallow {
				childSize, exists := idx.GetFolderSize(childIndexPath)
				if exists {
					totalSize += childSize
				} else {
					childRealPath := realPath + "/" + childName
					childSize := idx.calculateDirectorySize(childRealPath, childIndexPath, false, nil)
					totalSize += childSize
					idx.SetFolderSize(childIndexPath, childSize)
				}
			} else {
				childRealPath := realPath + "/" + childName
				childSize := idx.calculateDirectorySize(childRealPath, childIndexPath, false, nil)
				totalSize += childSize
			}
		} else {
			if shallow {
				// Use configured size calculation method
				childRealPath := realPath + "/" + file.Name()
				fileSize := getDiskUsage(file, childRealPath, idx.Config.UseLogicalSize)
				totalSize += uint64(fileSize)
			} else {
				childRealPath := realPath + "/" + file.Name()
				childIndexPath := indexPath + file.Name()
				size, shouldCount := idx.handleFile(file, childIndexPath, childRealPath, false, nil)
				if shouldCount {
					totalSize += size
				}
			}
		}
	}

	// Ensure minimum 4KB for directories only in disk usage mode
	if !idx.Config.UseLogicalSize && totalSize < 4096 {
		totalSize = 4096
	}

	return totalSize
}

// updateFolderSizeAndParents updates a folder size and propagates the change to all parents
func (idx *Index) updateFolderSizeAndParents(path string, newSize uint64, previousSize uint64, updateTarget bool) {
	// Calculate the delta
	var sizeDelta int64
	if newSize >= previousSize {
		sizeDelta = int64(newSize - previousSize)
	} else {
		sizeDelta = -int64(previousSize - newSize)
	}

	if sizeDelta == 0 && !updateTarget {
		return // No change, nothing to propagate
	}

	idx.folderSizesMu.Lock()
	defer idx.folderSizesMu.Unlock()

	// Optionally update the target folder itself
	if updateTarget {
		idx.folderSizes[path] = newSize
		idx.folderSizesUnsynced[path] = struct{}{}
	}

	if sizeDelta == 0 {
		return // No parent updates needed
	}

	// Walk up the parent chain and update all parents
	currentPath := path
	for {
		parentDir := utils.GetParentDirectoryPath(currentPath)
		if parentDir == "" {
			break // Reached the top
		}
		if parentDir != "/" {
			parentDir = utils.AddTrailingSlashIfNotExists(parentDir)
		}

		// Update parent size with delta
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
		idx.folderSizesUnsynced[parentDir] = struct{}{} // Mark parent as unsynced

		// Move up to the next parent
		currentPath = parentDir
	}
}

// SetFolderSize sets the size for a directory in the in-memory map
func (idx *Index) SetFolderSize(path string, newSize uint64) {
	idx.folderSizesMu.Lock()
	defer idx.folderSizesMu.Unlock()
	idx.folderSizes[path] = newSize
	idx.folderSizesUnsynced[path] = struct{}{} // Mark as changed for next DB sync
}

// GetFolderSize retrieves the size for a directory from the in-memory map
func (idx *Index) GetFolderSize(path string) (uint64, bool) {
	idx.folderSizesMu.RLock()
	defer idx.folderSizesMu.RUnlock()
	size, exists := idx.folderSizes[path]
	return size, exists
}

// GetFolderSizeForDisplay retrieves folder size with appropriate formatting based on config
func (idx *Index) GetFolderSizeForDisplay(path string) int64 {
	size, exists := idx.GetFolderSize(path)
	if !exists {
		// No cached size - return appropriate default
		if idx.Config.UseLogicalSize {
			return 0 // Logical mode: empty directory = 0
		}
		return 4096 // Disk usage mode: minimum 4KB
	}

	// Apply 4KB minimum in disk usage mode
	if !idx.Config.UseLogicalSize && size < 4096 {
		return 4096
	}

	return int64(size)
}

// getFileSizeForDisplay returns file size with appropriate calculation based on config
func (idx *Index) getFileSizeForDisplay(file os.FileInfo, realPath string) int64 {
	if idx.Config.UseLogicalSize {
		// Logical size mode: return actual bytes
		return file.Size()
	}

	// Disk usage mode: use getDiskUsage helper
	return getDiskUsage(file, realPath, false)
}

// RecursiveUpdateDirSizes is now a wrapper for backwards compatibility
// This maintains API compatibility while using the new optimized approach
func (idx *Index) RecursiveUpdateDirSizes(path string, previousSize uint64) {
	currentSize, exists := idx.GetFolderSize(path)
	if !exists {
		logger.Debugf("[FOLDER_SIZE] Path %s not found in folderSizes map", path)
		return
	}

	if currentSize != previousSize {
		idx.updateFolderSizeAndParents(path, currentSize, previousSize, false) // updateTarget=false, only update parents
	}
}

// SyncFolderSizesToDB syncs in-memory folder sizes to the database after a scan completes
// Only syncs folders marked as dirty (changed) and processes in batches to avoid large SQL statements
func (idx *Index) SyncFolderSizesToDB() error {

	idx.folderSizesMu.Lock()

	// Collect only dirty (changed) folder sizes
	sizesToSync := make(map[string]uint64)
	for path := range idx.folderSizesUnsynced {
		if size, exists := idx.folderSizes[path]; exists {
			sizesToSync[path] = size
		}
	}
	idx.folderSizesMu.Unlock()

	if len(sizesToSync) == 0 {
		logger.Debugf("[FOLDER_SIZE_SYNC] No dirty folder sizes to sync")
		return nil
	}

	// Process in batches of 1000 to avoid massive SQL statements
	const batchSize = 1000
	paths := make([]string, 0, len(sizesToSync))
	for path := range sizesToSync {
		paths = append(paths, path)
	}

	totalUpdated := int64(0)
	for i := 0; i < len(paths); i += batchSize {
		end := i + batchSize
		if end > len(paths) {
			end = len(paths)
		}

		batchPaths := paths[i:end]
		batchMap := make(map[string]uint64, len(batchPaths))
		for _, path := range batchPaths {
			batchMap[path] = sizesToSync[path]
		}

		rowsUpdated, err := idx.db.UpdateFolderSizesIfChanged(idx.Name, batchMap)
		if err != nil {
			logger.Errorf("[FOLDER_SIZE_SYNC] Failed to update folder sizes batch %d-%d: %v", i, end, err)
			return err
		}
		totalUpdated += rowsUpdated

		// Remove synced paths from unsynced map
		idx.folderSizesMu.Lock()
		for _, path := range batchPaths {
			delete(idx.folderSizesUnsynced, path)
		}
		idx.folderSizesMu.Unlock()

		if rowsUpdated > 0 {
			logger.Debugf("[FOLDER_SIZE_SYNC] Batch %d-%d: updated %d folders (skipped %d unchanged)",
				i, end, rowsUpdated, len(batchPaths)-int(rowsUpdated))
		}
	}

	if totalUpdated > 0 {
		logger.Infof("[FOLDER_SIZE_SYNC] Updated %d folder sizes (skipped %d unchanged)",
			totalUpdated, len(sizesToSync)-int(totalUpdated))
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
	if adjustedPath == "/" {
		return true
	}
	rules := idx.Config.ResolvedConditionals
	if rules == nil {
		return false
	}

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
		// Check if file is inside a viewable folder
		// 1. Check specific file rules FIRST to allow override (e.g. excluded file in viewable folder)
		if rule, exists := rules.FileNames[baseName]; exists {
			return rule.Viewable
		}
		if rule, exists := rules.FilePaths[adjustedPath]; exists {
			return rule.Viewable
		}
		for path, rule := range rules.FilePaths {
			if strings.HasPrefix(adjustedPath, path) {
				return rule.Viewable
			}
		}
		for _, rule := range rules.FileEndsWith {
			if strings.HasSuffix(baseName, rule.FileEndsWith) {
				return rule.Viewable
			}
		}
		for _, rule := range rules.FileStartsWith {
			if strings.HasPrefix(baseName, rule.FileStartsWith) {
				return rule.Viewable
			}
		}

		// 2. If no file rules matched, check if we inherit visibility from a parent folder
		for _, rule := range rules.FolderPaths {
			if strings.HasPrefix(adjustedPath, rule.FolderPath) && rule.Viewable {
				return true
			}
		}
	}
	return false
}

// IsIndexable checks if a path should be indexed (not skipped).
// This is an exported wrapper around shouldSkip for use by external callers.
// fileInfo is the os.FileInfo for the path, used to determine if it's hidden.
func (idx *Index) IsIndexable(fileInfo os.FileInfo, fullCombined string) bool {
	if fileInfo == nil {
		return false
	}
	hidden := isHidden(fileInfo, idx.Path+fullCombined)
	baseName := fileInfo.Name()
	return !idx.shouldSkip(fileInfo.IsDir(), hidden, fullCombined, baseName, Options{
		CheckViewable: false, // Don't check viewable here - that's handled separately
	})
}

func (idx *Index) shouldSkip(isDir bool, isHidden bool, fullCombined, baseName string, opts Options) bool {
	rules := idx.Config.ResolvedConditionals
	if rules == nil {
		rules = &settings.ResolvedConditionalsConfig{}
	}
	if fullCombined == "/" {
		return false
	}
	if idx.Config.DisableIndexing {
		return !opts.CheckViewable
	}

	if isDir && opts.IsRoutineScan {
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
