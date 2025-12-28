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
	"github.com/gtsteffaniak/go-push/push"
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
	FoundHardLinks   map[string]uint64 // hardlink path -> size
	processedInodes  map[uint64]struct{}
	previousNumDirs  uint64               // Track previous NumDirs to use when scan in progress (computed value is 0)
	previousNumFiles uint64               // Track previous NumFiles to use when scan in progress (computed value is 0)
	batchItems       []*iteminfo.FileInfo // Accumulates items during a scan for bulk insert
	scanners         map[string]*Scanner  // path -> scanner
	mock             bool
	mu               sync.RWMutex
	childScanMutex   sync.Mutex // Serializes child scanner execution (only one child scanner runs at a time)
	// Delayed parent size updates: accumulate deltas and batch update after 1 second of inactivity
	pendingParentSizeDeltas map[string]int64      // path -> accumulated delta
	parentSizeFlushMutex    sync.Mutex            // Mutex for parent size flush operations
	parentSizePacer         *push.Pacer[struct{}] // Debounced pacer to trigger batch flushes (handles rate limiting)
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
		mock:                    mock,
		Source:                  *source,
		db:                      indexDB, // Use shared database
		processedInodes:         make(map[uint64]struct{}),
		FoundHardLinks:          make(map[string]uint64),
		batchItems:              nil, // Don't initialize batch for direct use - only scanners do this
		pendingParentSizeDeltas: make(map[string]int64),
		scanUpdatedPaths:        make(map[string]bool),
	}

	// Initialize go-push pacer for debounced parent size updates
	// Debounce mode: waits 1 second after last update before processing
	config := push.Config{
		Mode:     push.ModeDebounce,
		Interval: 1 * time.Second,
	}
	newIndex.parentSizePacer = push.New[struct{}](config)

	// Start consumer goroutine to process debounced flushes
	go newIndex.processParentSizeUpdates()
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
func (idx *Index) indexDirectory(adjustedPath string, config actionConfig) (int64, bool, error) {
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
	dirFileInfo, err2 := idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, config)
	if err2 != nil {
		return 0, false, err2
	}
	idx.UpdateMetadata(dirFileInfo)
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
		realSize, _ := idx.handleFile(dirInfo, adjustedPath, realPath, false)
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
	})
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
	return response, nil

}

func (idx *Index) GetDirInfo(dirInfo *os.File, stat os.FileInfo, realPath, adjustedPath, combinedPath string, config actionConfig) (*iteminfo.FileInfo, error) {
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
				subdirSize, subdirHasPreview, err := idx.indexDirectory(dirPath, config)
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
			size, shouldCountSize := idx.handleFile(file, fullCombined, realFilePath, config.IsRoutineScan)
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
	if totalSize == 0 && idx.Config.Conditionals.ZeroSizeFolders {
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

	logger.Debugf("[REFRESH] RefreshFileInfo called for %s (IsDir=%v, Recursive=%v)", targetPath, opts.IsDir, opts.Recursive)

	previousInfo, previousExists := idx.GetMetadataInfo(targetPath, true)
	var previousSize int64
	if previousExists {
		previousSize = previousInfo.Size
	}

	realPath, _, err := idx.GetRealPath(targetPath)
	if err != nil {
		logger.Errorf("[REFRESH] Failed to get real path for %s: %v", targetPath, err)
		return err
	}

	// Check if directory still exists on filesystem
	_, err = os.Stat(realPath)
	if err != nil {
		idx.DeleteMetadata(targetPath, true, false)
		return nil
	}

	// RefreshFileInfo no longer needs the scan mutex - we use timestamp-based conflict detection
	// This allows API requests to work during scans without waiting
	// The timestamp check in UpdateDirectorySizeIfStale will prevent overwriting scan updates

	config := actionConfig{
		Quick:     true,
		Recursive: opts.Recursive,
	}
	newSize, _, err := idx.indexDirectory(targetPath, config)
	if err != nil {
		logger.Errorf("[REFRESH] indexDirectory failed for %s: %v", targetPath, err)
		return err
	}

	newInfo, exists := idx.GetMetadataInfo(targetPath, true)
	if !exists {
		return nil
	}

	sizeDelta := newInfo.Size - previousSize

	// Check if calculated size matches DB size - if not, there might be a race condition
	// This can happen during active scanning when the DB is being updated concurrently
	// Only log as warning if the difference is significant (>1% or >1MB)
	sizeDiff := newSize - newInfo.Size
	if sizeDiff < 0 {
		sizeDiff = -sizeDiff
	}
	if sizeDiff > 0 {
		// Log warning only if difference is significant (more than 1% or 1MB)
		if sizeDiff > newSize/100 || sizeDiff > 1024*1024 {
			logger.Warningf("[REFRESH] Size mismatch for %s: calculated size (%d) != DB size (%d), diff=%d (may be due to concurrent scan)",
				targetPath, newSize, newInfo.Size, sizeDiff)
		}
	}

	if sizeDelta != 0 {
		if opts.IsDir {
			// Directory operations: update parent sizes immediately
			// For deletions (negative delta), verify the delta won't cause negative parent sizes
			if sizeDelta < 0 {
				// Check if any parent would go negative - if so, recalculate instead
				parentPaths := []string{}
				currentPath := targetPath
				for {
					parentDir := utils.GetParentDirectoryPath(currentPath)
					if parentDir == "" {
						break
					}
					if parentDir != "/" {
						parentDir = utils.AddTrailingSlashIfNotExists(parentDir)
					}
					parentPaths = append(parentPaths, parentDir)
					if parentDir == "/" {
						break
					}
					currentPath = parentDir
				}

				// Quick check: if any parent size would go negative, recalculate from filesystem
				// Recalculate from the immediate parent up to ensure child sizes are accurate
				parentInfos, err := idx.db.GetItemsByPaths(idx.Name, parentPaths)
				if err == nil {
					// Check parents from deepest to shallowest (reverse order)
					for i := len(parentPaths) - 1; i >= 0; i-- {
						path := parentPaths[i]
						if info, exists := parentInfos[path]; exists {
							if info.Size+sizeDelta < 0 {
								logger.Warningf("[REFRESH] Deletion would cause negative size for parent %s (%d + %d). Recalculating from filesystem.",
									path, info.Size, sizeDelta)
								// Recalculate parent recursively to ensure child directory sizes are accurate
								// This is critical when multiple deletions happen rapidly
								parentOpts := utils.FileOptions{Path: path, IsDir: true, Recursive: true}
								if err := idx.RefreshFileInfo(parentOpts); err != nil {
									logger.Errorf("[REFRESH] Failed to recalculate parent %s: %v", path, err)
								}
								return nil // Skip delta-based update, recalculation handled it
							}
						}
					}
				}
			}

			idx.updateParentDirSizesBatched(targetPath, sizeDelta)
		} else {
			// File operations: schedule delayed batch update to handle rapid-fire uploads
			// This accumulates parent size deltas and flushes them after 1 second of inactivity
			idx.scheduleDelayedParentSizeUpdate(targetPath, sizeDelta)
		}
	}

	return nil
}

// updateParentDirSizesBatched updates all parent directory sizes in a single batch operation.
// This queries all parent paths from the database, updates their sizes, and batch updates them.
func (idx *Index) updateParentDirSizesBatched(startPath string, sizeDelta int64) {
	if sizeDelta == 0 {
		logger.Debugf("[PARENT_SIZE] Skipping update: sizeDelta is 0 for %s", startPath)
		return
	}

	logger.Infof("[PARENT_SIZE] Starting parent size update for %s with delta=%d", startPath, sizeDelta)

	// Check batch state before updating parent sizes
	idx.mu.RLock()
	hasBatchItems := len(idx.batchItems) > 0
	batchSize := 0
	if hasBatchItems {
		batchSize = len(idx.batchItems)
	}
	idx.mu.RUnlock()

	if hasBatchItems {
		logger.Warningf("[PARENT_SIZE] WARNING: Batch items exist (%d items) when updating parent sizes for %s - may read stale data!",
			batchSize, startPath)
	}

	parentPaths := []string{}
	currentPath := startPath

	for {
		parentDir := utils.GetParentDirectoryPath(currentPath)
		if parentDir == "" {
			break
		}
		if parentDir != "/" {
			parentDir = utils.AddTrailingSlashIfNotExists(parentDir)
		}
		parentPaths = append(parentPaths, parentDir)
		if parentDir == "/" {
			break
		}
		currentPath = parentDir
	}

	if len(parentPaths) == 0 {
		logger.Debugf("[PARENT_SIZE] No parent paths found for %s", startPath)
		return
	}

	logger.Debugf("[PARENT_SIZE] Found %d parent paths to update: %v", len(parentPaths), parentPaths)

	// Accumulate deltas for all parent paths - go-push will debounce and batch process them
	idx.parentSizeFlushMutex.Lock()
	for _, path := range parentPaths {
		idx.pendingParentSizeDeltas[path] += sizeDelta
	}
	idx.parentSizeFlushMutex.Unlock()

	// Trigger debounced flush - go-push will wait 1 second after last update before processing
	// This automatically handles rate limiting and batching of rapid updates
	idx.parentSizePacer.Push(struct{}{})
	logger.Debugf("[PARENT_SIZE] Accumulated deltas for %d parent paths, will flush after 1 second of inactivity", len(parentPaths))
}

// scheduleDelayedParentSizeUpdate accumulates parent size deltas and schedules a batch update
// after 1 second of inactivity. This prevents rapid-fire updates during bulk file operations.
func (idx *Index) scheduleDelayedParentSizeUpdate(startPath string, sizeDelta int64) {
	if sizeDelta == 0 {
		return
	}

	idx.parentSizeFlushMutex.Lock()
	defer idx.parentSizeFlushMutex.Unlock()

	// Get all parent paths for this path
	parentPaths := []string{}
	currentPath := startPath
	for {
		parentDir := utils.GetParentDirectoryPath(currentPath)
		if parentDir == "" {
			break
		}
		if parentDir != "/" {
			parentDir = utils.AddTrailingSlashIfNotExists(parentDir)
		}
		parentPaths = append(parentPaths, parentDir)
		if parentDir == "/" {
			break
		}
		currentPath = parentDir
	}

	// Accumulate deltas for each parent path
	for _, path := range parentPaths {
		idx.pendingParentSizeDeltas[path] += sizeDelta
	}

	// Trigger debounced flush - go-push will wait 1 second after last update
	// This automatically handles rapid-fire updates by batching them
	idx.parentSizePacer.Push(struct{}{})
}

// processParentSizeUpdates is the consumer goroutine that processes debounced parent size updates.
// It receives triggers from the go-push pacer after 1 second of inactivity and flushes accumulated deltas.
func (idx *Index) processParentSizeUpdates() {
	for range idx.parentSizePacer.Updates() {
		// Process all accumulated deltas after debounce period
		idx.flushPendingParentSizeUpdates()
	}
}

// FlushPendingParentSizeUpdates flushes any accumulated parent size updates immediately.
// This should be called before scans to ensure data consistency.
func (idx *Index) FlushPendingParentSizeUpdates() {
	idx.flushPendingParentSizeUpdates()
}

// flushPendingParentSizeUpdates applies all accumulated parent size deltas in a single batch operation
func (idx *Index) flushPendingParentSizeUpdates() {
	idx.parentSizeFlushMutex.Lock()
	defer idx.parentSizeFlushMutex.Unlock()

	if len(idx.pendingParentSizeDeltas) == 0 {
		return
	}

	// Get all paths that need updating
	pathsToUpdate := make([]string, 0, len(idx.pendingParentSizeDeltas))
	for path := range idx.pendingParentSizeDeltas {
		pathsToUpdate = append(pathsToUpdate, path)
	}

	// Query all parent directories from database in a single query
	parentInfos, err := idx.db.GetItemsByPaths(idx.Name, pathsToUpdate)
	if err != nil {
		logger.Errorf("[PARENT_SIZE] Failed to query parent directories for batch update: %v", err)
		// Clear pending deltas on error to prevent accumulation
		idx.pendingParentSizeDeltas = make(map[string]int64)
		return
	}

	// Build map of path -> size delta for batch update
	// Only include paths that actually exist in the database
	pathSizeUpdates := make(map[string]int64)
	for path, delta := range idx.pendingParentSizeDeltas {
		if info, exists := parentInfos[path]; exists {
			newSize := info.Size + delta
			// Prevent negative sizes
			if newSize < 0 {
				logger.Warningf("[PARENT_SIZE] Skipping update for %s: would result in negative size (%d + %d)",
					path, info.Size, delta)
				continue
			}
			pathSizeUpdates[path] = delta
			logger.Debugf("[PARENT_SIZE] Will update %s: %d -> %d (delta=%d)",
				path, info.Size, newSize, delta)
		} else {
			logger.Debugf("[PARENT_SIZE] Parent path %s not found in database, skipping", path)
		}
	}

	// Clear pending deltas before DB operation
	idx.pendingParentSizeDeltas = make(map[string]int64)

	if len(pathSizeUpdates) == 0 {
		logger.Debugf("[PARENT_SIZE] No valid parent paths to update in batch flush")
		return
	}

	// Batch update all parent sizes in a single transaction
	logger.Infof("[PARENT_SIZE] Batch updating %d parent directory sizes (delayed flush)", len(pathSizeUpdates))
	err = idx.db.BulkUpdateSizes(idx.Name, pathSizeUpdates)
	if err != nil {
		logger.Errorf("[PARENT_SIZE] Failed to batch update parent directory sizes: %v", err)
	} else {
		logger.Debugf("[PARENT_SIZE] Successfully batch updated %d parent directory sizes", len(pathSizeUpdates))
	}
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

	if idx.Config.Conditionals.IgnoreHidden && isHidden {
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
