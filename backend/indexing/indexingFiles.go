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

// Options holds all configuration options for indexing and filesystem operations
type Options struct {
	// Indexing operation options
	Recursive     bool // whether to recursively index subdirectories
	CheckViewable bool // whether to check if the path has viewable:true (for API access checks)
	IsRoutineScan bool // whether this is a routine/scheduled scan (vs initial indexing)

	// Filesystem info retrieval options
	SkipExtendedAttrs bool // Skip hasPreview and other extended attributes
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
	ScanTime        int       `json:"scanDurationSeconds"` // Unified scan time metric
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

// GetScanTime calculates the total scan time by summing all scanner values
func (idx *Index) GetScanTime() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.getScanTimeUnlocked()
}

// getScanTimeUnlocked calculates ScanTime without acquiring lock (assumes lock is already held)
func (idx *Index) getScanTimeUnlocked() int {
	var total = 0
	for _, scanner := range idx.scanners {
		total += scanner.scanTime
	}
	return total
}

// GetComplexity calculates the complexity based on scan time and number of directories
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

	totalScanTime := idx.getScanTimeUnlocked()
	totalDirs := idx.getNumDirsUnlocked()
	calculatedComplexity := calculateComplexity(totalScanTime, totalDirs)

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

// PathContext contains all resolved information about a path
// Calculated ONCE to avoid duplicate stat calls and redundant checks
type PathContext struct {
	IndexPath string      // normalized path (dirs have trailing /)
	RealPath  string      // resolved filesystem path
	BaseName  string      // file/folder name only
	IsDir     bool        // directory or file
	IsHidden  bool        // starts with . or hidden attribute
	IsSymlink bool        // is symbolic link
	FileInfo  os.FileInfo // from stat call
}

// FileInfoRequest specifies what information to retrieve
type FileInfoRequest struct {
	IndexPath      string
	FollowSymlinks bool
	ShowHidden     bool
	Expand         bool // get child items for directories
	IsRoutineScan  bool // scanner vs API call
}

// resolvePathContext resolves all path characteristics in a SINGLE stat call
// This eliminates duplicate stat/lstat calls and redundant isHidden/isSymlink checks
func (idx *Index) resolvePathContext(indexPath string, followSymlinks bool) (*PathContext, error) {
	realPath := filepath.Join(idx.Path, indexPath)

	// ONE filesystem stat call
	fileInfo, err := os.Lstat(realPath)
	if err != nil {
		return nil, err
	}

	isSymlink := fileInfo.Mode()&os.ModeSymlink != 0

	// Resolve symlink if requested
	if isSymlink && followSymlinks {
		realPath, err = filepath.EvalSymlinks(realPath)
		if err != nil {
			return nil, err
		}
		fileInfo, err = os.Stat(realPath)
		if err != nil {
			return nil, err
		}
	}

	isDir := fileInfo.IsDir()

	// Normalize path (directories get trailing slash)
	normalizedPath := indexPath
	if isDir && !strings.HasSuffix(indexPath, "/") {
		normalizedPath += "/"
	}

	baseName := filepath.Base(strings.TrimSuffix(normalizedPath, "/"))
	if normalizedPath == "/" {
		baseName = filepath.Base(idx.Path)
	}

	return &PathContext{
		IndexPath: normalizedPath,
		RealPath:  realPath,
		BaseName:  baseName,
		IsDir:     isDir,
		IsHidden:  IsHidden(realPath),
		IsSymlink: isSymlink,
		FileInfo:  fileInfo,
	}, nil
}

// evaluateIndexRules checks if a path should be accessible based on index rules
// Returns whether the path is viewable and/or indexable
// Does NOT check user permissions (that's handled in the API layer)
func (idx *Index) evaluateIndexRules(ctx *PathContext, isRoutineScan bool) (isViewable bool, isIndexable bool) {
	isViewable = idx.IsViewable(ctx.IsDir, ctx.IndexPath, ctx.IsSymlink)
	shouldSkip := idx.ShouldSkip(ctx.IsDir, ctx.IndexPath, ctx.IsHidden, ctx.IsSymlink, isRoutineScan)
	isIndexable = !shouldSkip
	return isViewable, isIndexable
}

// GetFileInfo is the unified entry point for retrieving file/directory information
// It applies index rules (IsViewable/ShouldSkip) but does NOT check user permissions
// User permission checking happens in the API layer (FileInfoFaster)
func (idx *Index) GetFileInfo(req FileInfoRequest) (*iteminfo.FileInfo, error) {
	// 1. Resolve path context (single stat call)
	ctx, err := idx.resolvePathContext(req.IndexPath, req.FollowSymlinks)
	if err != nil {
		return nil, err
	}

	// 2. Apply index rules
	isViewable, isIndexable := idx.evaluateIndexRules(ctx, req.IsRoutineScan)

	// Path must be either viewable OR indexable
	if !isViewable && !isIndexable {
		return nil, errors.ErrNotIndexed
	}

	// 3. Return appropriate info
	if !ctx.IsDir {
		// Single file
		return idx.getFileInfoFromContext(ctx, isIndexable)
	}

	// Directory
	if req.Expand {
		// Get directory with children
		return idx.getDirInfoFromContext(ctx, isViewable, isIndexable, req)
	}

	// Just directory metadata (no children)
	return idx.getBasicDirInfo(ctx), nil
}

// getFileInfoFromContext returns info for a single file
func (idx *Index) getFileInfoFromContext(ctx *PathContext, isIndexable bool) (*iteminfo.FileInfo, error) {
	var size uint64
	if isIndexable {
		// Use indexed size
		size, _ = idx.handleFile(ctx.FileInfo, ctx.IndexPath, false, nil)
	} else {
		// Use filesystem size
		size = uint64(ctx.FileInfo.Size())
	}

	fileInfo := &iteminfo.FileInfo{
		Path: ctx.IndexPath,
		ItemInfo: iteminfo.ItemInfo{
			Name:    ctx.BaseName,
			Size:    int64(size),
			ModTime: ctx.FileInfo.ModTime(),
			Hidden:  ctx.IsHidden,
		},
	}
	fileInfo.DetectType(ctx.RealPath, false)

	// Set preview flags
	setFilePreviewFlags(&fileInfo.ItemInfo, ctx.RealPath)

	return fileInfo, nil
}

// getDirInfoFromContext gets full directory info using PathContext
func (idx *Index) getDirInfoFromContext(ctx *PathContext, isViewable, isIndexable bool, req FileInfoRequest) (*iteminfo.FileInfo, error) {
	dir, err := os.Open(ctx.RealPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	opts := Options{
		Recursive:         false,
		CheckViewable:     true,
		IsRoutineScan:     req.IsRoutineScan,
		SkipExtendedAttrs: !isIndexable, // Only fetch extended attrs if indexable
		FollowSymlinks:    req.FollowSymlinks,
		ShowHidden:        req.ShowHidden,
	}

	return idx.GetDirInfoCore(dir, ctx.FileInfo, ctx.IndexPath, opts, nil)
}

// getBasicDirInfo returns just directory metadata without children
func (idx *Index) getBasicDirInfo(ctx *PathContext) *iteminfo.FileInfo {
	return &iteminfo.FileInfo{
		Path: ctx.IndexPath,
		ItemInfo: iteminfo.ItemInfo{
			Name:    ctx.BaseName,
			Type:    "directory",
			Size:    0,
			ModTime: ctx.FileInfo.ModTime(),
			Hidden:  ctx.IsHidden,
		},
	}
}

// Define a function to recursively index files and directories
func (idx *Index) indexDirectory(indexPath string, opts Options, scanner *Scanner) (int64, bool, error) {
	if !strings.HasSuffix(indexPath, "/") {
		indexPath = indexPath + "/"
	}
	realPath := filepath.Join(idx.Path, indexPath)

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
	hidden := IsHidden(realPath)
	isSymlink := dirInfo.Mode()&os.ModeSymlink != 0
	if idx.ShouldSkip(dirInfo.IsDir(), indexPath, hidden, isSymlink, true) {
		return 0, false, errors.ErrNotIndexed
	}

	dirFileInfo, err2 := idx.GetDirInfo(dir, dirInfo, indexPath, opts, scanner)
	if err2 != nil {
		return 0, false, err2
	}
	idx.UpdateMetadata(dirFileInfo, scanner)

	// Always store the calculated directory size in the in-memory map
	// This ensures parent directories are updated after copy/move operations
	idx.SetFolderSize(indexPath, uint64(dirFileInfo.Size))

	return dirFileInfo.Size, dirFileInfo.HasPreview, nil
}

// GetFsInfoCore is the consolidated implementation for both GetFsInfo and GetFsInfoViewableOnly
func (idx *Index) GetFsInfoCore(indexPath string, opts Options) (*iteminfo.FileInfo, error) {
	// Handle symlinks if not following them
	realPath := filepath.Join(idx.Path, indexPath)
	if opts.FollowSymlinks {
		var err error
		realPath, err = filepath.EvalSymlinks(realPath)
		if err != nil {
			return nil, err
		}
	}

	dir, err := os.Open(realPath)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	dirInfo, err2 := dir.Stat()
	if err2 != nil {
		return nil, err2
	}
	baseName := filepath.Base(indexPath)
	if indexPath == "/" {
		baseName = filepath.Base(idx.Path)
	}

	// Handle file case
	if !dirInfo.IsDir() {
		// Check if item is accessible
		hidden := IsHidden(realPath)
		isSymlink := dirInfo.Mode()&os.ModeSymlink != 0
		isViewable := idx.IsViewable(dirInfo.IsDir(), indexPath, isSymlink)
		isSkipped := idx.ShouldSkip(dirInfo.IsDir(), indexPath, hidden, isSymlink, false)

		// Deny access if not viewable AND skipped
		if opts.CheckViewable {
			if !isViewable {
				return nil, errors.ErrNotIndexed
			}
		} else if isSkipped {
			return nil, errors.ErrNotIndexed
		}

		realSize, _ := idx.handleFile(dirInfo, indexPath, false, nil)
		fileInfo := &iteminfo.FileInfo{
			Path: indexPath,
			ItemInfo: iteminfo.ItemInfo{
				Name:    baseName,
				Size:    int64(realSize),
				ModTime: dirInfo.ModTime(),
			},
		}
		fileInfo.DetectType(realPath, false)
		// Set preview flags unless explicitly skipped
		if !opts.SkipExtendedAttrs {
			setFilePreviewFlags(&fileInfo.ItemInfo, realPath)
		}
		return fileInfo, nil
	}

	// Handle directory case
	response, err := idx.GetDirInfoCore(dir, dirInfo, indexPath, opts, nil)
	if err != nil {
		return nil, err
	}

	// Handle file-in-directory case
	if !dirInfo.IsDir() {
		for _, item := range response.Files {
			if item.Name == baseName {
				return &iteminfo.FileInfo{
					Path:     utils.JoinPathAsUnix(indexPath, item.Name),
					ItemInfo: item.ItemInfo,
				}, nil
			}
		}
		return nil, fmt.Errorf("file not found in directory: %s", indexPath)
	}

	return response, nil
}

// GetFsInfo returns filesystem information with index checks and extended attributes
func (idx *Index) GetFsInfo(adjustedPath string, followSymlinks bool, showHidden bool) (*iteminfo.FileInfo, error) {
	return idx.GetFsInfoCore(adjustedPath, Options{
		Recursive:         false,
		CheckViewable:     true,
		IsRoutineScan:     false,
		SkipExtendedAttrs: false,
		FollowSymlinks:    followSymlinks,
		ShowHidden:        showHidden,
	})
}

// GetFsInfoViewableOnly returns filesystem information for viewable-only paths (not indexed)
func (idx *Index) GetFsInfoViewableOnly(adjustedPath string, followSymlinks bool, showHidden bool) (*iteminfo.FileInfo, error) {
	return idx.GetFsInfoCore(adjustedPath, Options{
		Recursive:         false,
		CheckViewable:     true,
		IsRoutineScan:     false,
		SkipExtendedAttrs: true,
		FollowSymlinks:    followSymlinks,
		ShowHidden:        showHidden,
	})
}

// fetchExtendedAttributes fetches hasPreview for the current directory and batch fetches for subdirectories
func (idx *Index) fetchExtendedAttributes(indexPath string, files []os.FileInfo, opts Options) (bool, map[string]bool) {
	hasPreview := false
	subdirHasPreviewMap := make(map[string]bool)

	if opts.SkipExtendedAttrs || opts.Recursive {
		return hasPreview, subdirHasPreviewMap
	}

	// Fetch hasPreview for current directory
	realDirInfo, exists := idx.GetMetadataInfo(indexPath, true, true)
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
		if indexPath == "/" {
			if !idx.shouldInclude(baseName) {
				continue
			}
		}
		if omitList[baseName] {
			continue
		}
		dirPath := indexPath + baseName
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

// processDirectoryItem processes a directory item and returns the itemInfo, size, and whether it should be counted
func (idx *Index) processDirectoryItem(file os.FileInfo, indexPath string, subdirHasPreviewMap map[string]bool, opts Options, scanner *Scanner) (*iteminfo.ItemInfo, int64, bool) {
	dirPath := indexPath + file.Name()

	// Check NeverWatchPaths for recursive scans (scanner only)
	if opts.IsRoutineScan && idx.IsNeverWatchPath(indexPath) {
		return nil, 0, false
	}

	// Skip non-indexable dirs
	if omitList[file.Name()] {
		return nil, 0, false
	}

	itemInfo := &iteminfo.ItemInfo{
		Name:    file.Name(),
		ModTime: file.ModTime(),
		Hidden:  IsHidden(utils.JoinPathAsUnix(idx.Path, indexPath, file.Name())),
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

	// Non-recursive: get folder size and hasPreview from cache
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

	return itemInfo, itemInfo.Size, true
}

// processFileItem processes a file item and returns the itemInfo, size, shouldCount, and whether it bubbles up hasPreview
func (idx *Index) processFileItem(file os.FileInfo, indexPath string, opts Options, scanner *Scanner) (*iteminfo.ItemInfo, int64, bool, bool) {

	fullCombined := utils.JoinPathAsUnix(idx.Path, indexPath, file.Name())
	itemInfo := &iteminfo.ItemInfo{
		Name:    file.Name(),
		ModTime: file.ModTime(),
		Hidden:  IsHidden(fullCombined),
	}
	itemInfo.DetectType(fullCombined, false)

	// For API calls (non-recursive, no scanner), use appropriate size calculation
	// For scanning (recursive or with scanner), use handleFile for hardlink detection
	var size uint64
	var shouldCountSize bool
	if !opts.Recursive && scanner == nil {
		// API call: use helper function for config-aware size calculation
		size = uint64(idx.getFileSizeForDisplay(file, fullCombined))
		shouldCountSize = true
	} else {
		// Scanning: use handleFile for accurate size and hardlink detection
		size, shouldCountSize = idx.handleFile(file, indexPath, opts.IsRoutineScan, scanner)
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
			setFilePreviewFlags(itemInfo, fullCombined)
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

// GetDirInfoCore is the consolidated implementation for both GetDirInfo and GetDirInfoViewableOnly
func (idx *Index) GetDirInfoCore(dirInfo *os.File, stat os.FileInfo, indexPath string, opts Options, scanner *Scanner) (*iteminfo.FileInfo, error) {
	if !strings.HasSuffix(indexPath, "/") {
		indexPath = indexPath + "/"
	}
	files, err := dirInfo.Readdir(-1)
	if err != nil {
		return nil, err
	}
	// Fetch extended attributes (hasPreview for current dir and subdirs)
	hasPreview, subdirHasPreviewMap := idx.fetchExtendedAttributes(indexPath, files, opts)

	baseName := filepath.Base(indexPath)
	if indexPath == "/" {
		baseName = filepath.Base(idx.Path)
	}

	var totalSize int64
	fileInfos := []iteminfo.ExtendedItemInfo{}
	dirInfos := []iteminfo.ItemInfo{}
	processedCount := 0

	for _, file := range files {
		subfileBaseName := file.Name()
		isDir := iteminfo.IsDirectory(file)
		fullCombined := indexPath + subfileBaseName
		// Add trailing slash for directories to match normalized folder paths
		if isDir {
			fullCombined = fullCombined + "/"
		}
		hidden := IsHidden(utils.JoinPathAsUnix(idx.Path, indexPath, subfileBaseName))
		isSymlink := file.Mode()&os.ModeSymlink != 0

		// Check if item should be skipped
		shouldSkip := false

		// Check hidden files
		if !opts.ShowHidden && hidden {
			shouldSkip = true
		}

		// Check root include rules
		if !shouldSkip && indexPath == "/" && !idx.shouldInclude(subfileBaseName) {
			shouldSkip = true
		}

		// Check viewable/indexable rules
		if !shouldSkip {
			isViewable := idx.IsViewable(isDir, fullCombined, isSymlink)
			isSkipped := idx.ShouldSkip(isDir, fullCombined, hidden, isSymlink, opts.IsRoutineScan)
			if opts.CheckViewable {
				shouldSkip = !isViewable && isSkipped
			} else {
				shouldSkip = isSkipped
			}
		}
		if shouldSkip {
			continue
		}
		processedCount++

		if isDir {
			itemInfo, size, shouldCount := idx.processDirectoryItem(file, indexPath, subdirHasPreviewMap, opts, scanner)
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
			itemInfo, size, shouldCount, bubblesUp := idx.processFileItem(file, indexPath, opts, scanner)
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
	if idx.Config.ResolvedRules.IgnoreAllZeroSizeFolders && indexPath != "/" {
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
		Path:    indexPath,
		Files:   fileInfos,
		Folders: dirInfos,
	}
	dirFileInfo.ItemInfo = iteminfo.ItemInfo{
		Name:       baseName,
		Type:       "directory",
		Size:       totalSize,
		ModTime:    stat.ModTime(),
		HasPreview: hasPreview,
	}
	dirFileInfo.SortItems()

	return dirFileInfo, nil
}

// GetDirInfo returns directory information with index checks and extended attributes
func (idx *Index) GetDirInfo(dirInfo *os.File, stat os.FileInfo, indexPath string, opts Options, scanner *Scanner) (*iteminfo.FileInfo, error) {
	// Ensure filesystem options are set correctly for indexed paths
	opts.SkipExtendedAttrs = false
	return idx.GetDirInfoCore(dirInfo, stat, indexPath, opts, scanner)
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
	// For files, refresh the parent directory
	if !opts.IsDir {
		parentPath := idx.MakeIndexPath(filepath.Dir(opts.Path), true)
		return idx.RefreshDirectory(parentPath, false) // non-recursive for file changes
	}

	// For directories, use the recursive flag from opts
	indexPath := idx.MakeIndexPath(opts.Path, true)
	return idx.RefreshDirectory(indexPath, opts.Recursive)
}

// RefreshDirectory re-indexes a directory and updates its cached size
// Use recursive=true for copy/move operations to capture entire tree
// Use recursive=false for simple metadata updates (like after file edits)
func (idx *Index) RefreshDirectory(indexPath string, recursive bool) error {
	if !strings.HasSuffix(indexPath, "/") {
		indexPath = indexPath + "/"
	}

	realPath := filepath.Join(idx.Path, indexPath)

	// Check if directory exists
	dirInfo, err := os.Stat(realPath)
	if err != nil {
		// Directory deleted - clear from cache and update parents
		idx.folderSizesMu.Lock()
		previousSize, exists := idx.folderSizes[indexPath]
		delete(idx.folderSizes, indexPath)
		delete(idx.folderSizesUnsynced, indexPath)
		idx.folderSizesMu.Unlock()

		if exists && previousSize > 0 {
			idx.updateFolderSizeAndParents(indexPath, 0, previousSize, false)
		}
		return nil
	}

	if !dirInfo.IsDir() {
		return fmt.Errorf("path is not a directory: %s", indexPath)
	}

	// Check if excluded from indexing
	hidden := IsHidden(realPath)
	isSymlink := dirInfo.Mode()&os.ModeSymlink != 0
	if idx.ShouldSkip(true, indexPath, hidden, isSymlink, false) {
		return errors.ErrNotIndexed
	}

	// Use indexDirectory for proper recursive indexing
	opts := Options{
		Recursive:     recursive,
		IsRoutineScan: false,
		CheckViewable: false,
	}

	_, _, err = idx.indexDirectory(indexPath, opts, nil)
	return err
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

func IsHidden(realPath string) bool {
	if filepath.Base(realPath)[0] == '.' {
		return true
	}
	if runtime.GOOS == "windows" {
		return CheckWindowsHidden(realPath)
	}
	// Default behavior for non-Windows systems
	return false
}

// setFilePreviewFlags determines if a file should have a preview based on its type
func setFilePreviewFlags(fileInfo *iteminfo.ItemInfo, realPath string) {
	if fileInfo.Size == 0 {
		return
	}
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

// IsViewable checks if a path is viewable (allows FS access)
func (idx *Index) IsViewable(isDir bool, indexPath string, isSymlink bool) bool {
	if indexPath == "/" {
		return true
	}

	// Check symlinks
	if isSymlink && idx.Config.ResolvedRules.IgnoreAllSymlinks {
		return false
	}
	rules := idx.Config.ResolvedRules
	baseName := filepath.Base(indexPath)
	if indexPath == "/" {
		baseName = filepath.Base(idx.Path)
	}

	if isDir {
		// Check folder rules - return false if explicitly set to non-viewable
		if rule, exists := rules.FolderNames[baseName]; exists {
			// Cache this match in FolderPaths so children inherit via prefix matching
			if _, ok := rules.FolderPaths[indexPath]; !ok {
				rules.FolderPaths[indexPath] = rule
			}
			return rule.Viewable
		}
		if rule, exists := rules.FolderPaths[indexPath]; exists {
			return rule.Viewable
		}
		for path, rule := range rules.FolderPaths {
			if strings.HasPrefix(indexPath, path) {
				return rule.Viewable
			}
		}
		for _, rule := range rules.FolderEndsWith {
			if strings.HasSuffix(baseName, rule.FolderEndsWith) {
				// Cache this match in FolderPaths so children inherit
				if _, ok := rules.FolderPaths[indexPath]; !ok {
					rules.FolderPaths[indexPath] = rule
				}
				return rule.Viewable
			}
		}
		for _, rule := range rules.FolderStartsWith {
			if strings.HasPrefix(baseName, rule.FolderStartsWith) {
				// Cache this match in FolderPaths so children inherit
				if _, ok := rules.FolderPaths[indexPath]; !ok {
					rules.FolderPaths[indexPath] = rule
				}
				return rule.Viewable
			}
		}
	} else {
		// Check file rules
		if rule, exists := rules.FileNames[baseName]; exists {
			return rule.Viewable
		}
		if rule, exists := rules.FilePaths[indexPath]; exists {
			return rule.Viewable
		}
		for path, rule := range rules.FilePaths {
			if strings.HasPrefix(indexPath, path) {
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
		// Check if file inherits viewable status from parent folder
		for path, rule := range rules.FolderPaths {
			if strings.HasPrefix(indexPath, path) {
				return rule.Viewable
			}
		}
	}

	return true // Default: viewable unless explicitly set to false
}

// ShouldSkip checks if a path should be skipped from indexing
// isRoutineScan: true for scanner (no parent checks), false for API (check parents)
func (idx *Index) ShouldSkip(isDir bool, adjustedPath string, isHidden bool, isSymlink bool, isRoutineScan bool) bool {
	rules := idx.Config.ResolvedRules
	if adjustedPath == "/" {
		return false
	}
	if idx.Config.DisableIndexing {
		return true
	}
	if isHidden && rules.IgnoreAllHidden {
		return true
	}
	if isSymlink && rules.IgnoreAllSymlinks {
		return true
	}
	baseName := filepath.Base(adjustedPath)

	if isDir {
		// Check exact name match
		if _, ok := rules.FolderNames[baseName]; ok {
			return true
		}
		// Check exact path match
		if _, ok := rules.FolderPaths[adjustedPath]; ok {
			return true
		}
		// For API calls (not routine scan), check parent paths too
		if !isRoutineScan {
			for path := range rules.FolderPaths {
				if strings.HasPrefix(adjustedPath, path) {
					return true
				}
			}
		}
		// Check suffix match
		for _, rule := range rules.FolderEndsWith {
			if strings.HasSuffix(baseName, rule.FolderEndsWith) {
				return true
			}
		}
		// Check prefix match
		for _, rule := range rules.FolderStartsWith {
			if strings.HasPrefix(baseName, rule.FolderStartsWith) {
				return true
			}
		}
	} else {
		// Check exact name match
		if _, ok := rules.FileNames[baseName]; ok {
			return true
		}
		// Check exact path match
		if _, ok := rules.FilePaths[adjustedPath]; ok {
			return true
		}
		// For API calls (not routine scan), check parent paths too
		// Also check if file is inside a viewable-only folder
		if !isRoutineScan {
			for path, rule := range rules.FilePaths {
				if strings.HasPrefix(adjustedPath, path) && !rule.Viewable {
					return true
				}
			}
			// Check if file's parent folder is viewable (don't skip if parent allows viewing)
			for path, rule := range rules.FolderPaths {
				if strings.HasPrefix(adjustedPath, path) && !rule.Viewable {
					return true
				}
			}
		}
		// Check suffix match
		for _, rule := range rules.FileEndsWith {
			if strings.HasSuffix(baseName, rule.FileEndsWith) {
				return true
			}
		}
		// Check prefix match
		for _, rule := range rules.FileStartsWith {
			if strings.HasPrefix(baseName, rule.FileStartsWith) {
				return true
			}
		}
	}

	return false
}

// IsNeverWatchPath checks if a directory should not be re-indexed after the initial scan
// This is specifically for NeverWatchPaths - paths that are indexed once, then never watched again
// Returns true if the path should be skipped during routine scans (after initial indexing)
func (idx *Index) IsNeverWatchPath(adjustedPath string) bool {
	// Only apply NeverWatchPaths if we've completed at least one scan
	if idx.GetLastIndexed().IsZero() {
		return false // Initial scan - don't skip
	}

	rules := idx.Config.ResolvedRules
	if len(rules.NeverWatchPaths) == 0 {
		return false
	}

	// Check if this path is in NeverWatchPaths
	_, exists := rules.NeverWatchPaths[adjustedPath]
	return exists
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
	rules := idx.Config.ResolvedRules
	hasRules := false
	if len(rules.IncludeRootItems) > 0 {
		hasRules = true
		// Check with trailing slash since includeRootItems are normalized with trailing slashes
		if _, exists := rules.IncludeRootItems["/"+baseName+"/"]; exists {
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
			ScanTime:        scanner.scanTime,
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
