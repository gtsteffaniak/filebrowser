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
	Path            string    `json:"path"`
	IsRoot          bool      `json:"isRoot"`
	LastScanned     time.Time `json:"lastScanned"`
	Complexity      uint      `json:"complexity"` // 0-10 scale: 0=unknown, 1=simple, 2-6=normal, 7-9=complex, 10=highlyComplex
	CurrentSchedule int       `json:"currentSchedule"`
	QuickScanTime   int       `json:"quickScanTime"`
	FullScanTime    int       `json:"fullScanTime"`
	NumDirs         uint64    `json:"numDirs"`
	NumFiles        uint64    `json:"numFiles"`
}

// reduced index is json exposed to the client
type ReducedIndex struct {
	IdxName         string         `json:"name"`
	DiskUsed        uint64         `json:"used"`
	DiskTotal       uint64         `json:"total"`
	Status          IndexStatus    `json:"status"`
	NumDirs         uint64         `json:"numDirs"`
	NumFiles        uint64         `json:"numFiles"`
	NumDeleted      uint64         `json:"numDeleted"`
	LastIndexed     time.Time      `json:"-"`
	LastIndexedUnix int64          `json:"lastIndexedUnixTime"`
	QuickScanTime   int            `json:"quickScanDurationSeconds"`
	FullScanTime    int            `json:"fullScanDurationSeconds"`
	Complexity      uint           `json:"complexity"` // 0-10 scale: 0=unknown, 1=simple, 2-6=normal, 7-9=complex, 10=highlyComplex
	Scanners        []*ScannerInfo `json:"scanners,omitempty"`
}

type Index struct {
	ReducedIndex
	settings.Source `json:"-"`

	// Shared state (protected by mu)
	db                *dbsql.IndexDB       `json:"-"`
	FoundHardLinks    map[string]uint64    `json:"-"` // hardlink path -> size
	processedInodes   map[uint64]struct{}  `json:"-"`
	totalSize         uint64               `json:"-"`
	previousTotalSize uint64               `json:"-"` // Track previous totalSize for change detection
	batchItems        []*iteminfo.FileInfo `json:"-"` // Accumulates items during a scan for bulk insert
	isRoutineScan     bool                 `json:"-"` // Whether current scan is routine/scheduled (for retry logic)

	// Scanner management (new multi-scanner system)
	scanners             map[string]*Scanner `json:"-"` // path -> scanner
	scanMutex            sync.Mutex          `json:"-"` // Global scan mutex - only one scanner runs at a time
	activeScannerPath    string              `json:"-"` // Which scanner is currently running (for logging/status)
	runningScannerCount  int                 `json:"-"` // Tracks active scanners
	lastRootScanTime     time.Time           `json:"-"` // Last time root scanner completed - child scanners wait for this
	initialScanStartTime time.Time           `json:"-"` // When initial multi-scanner indexing started
	hasLoggedInitialScan bool                `json:"-"` // Whether we've logged the first complete round
	lastVacuumTime       time.Time           `json:"-"` // Last time VACUUM was performed on the database
	pendingFlushes       sync.WaitGroup      `json:"-"` // Tracks async progressive flush operations

	// Control
	mock       bool
	mu         sync.RWMutex
	wasIndexed bool
}

var (
	indexes      map[string]*Index
	indexesMutex sync.RWMutex
	indexDB      *dbsql.IndexDB // Shared database for all indexes
	indexDBOnce  sync.Once      // Ensures index DB is only created once
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

// InitializeIndexDB creates the shared index database for all sources.
// This should be called once at application startup before any sources are initialized.
func InitializeIndexDB() error {
	// clear all sql directory indexes
	sqlDir := filepath.Join(settings.Config.Server.CacheDir, "sql")
	err := os.RemoveAll(sqlDir)
	if err != nil {
		logger.Errorf("failed to clear sql directory: %v", err)
	}
	indexDBOnce.Do(func() {
		// Create a single shared database for all indexes
		indexDB, err = dbsql.NewIndexDB("all")
		if err != nil {
			logger.Errorf("failed to initialize index database: %v", err)
		} else {
			logger.Infof("Initialized shared index database for all sources")
		}
	})
	return err
}

// GetIndexDB returns the shared index database.
// Returns nil if InitializeIndexDB hasn't been called yet.
func GetIndexDB() *dbsql.IndexDB {
	return indexDB
}

// SetIndexDBForTesting sets the index database for testing purposes.
// This should only be used in tests to bypass InitializeIndexDB's permission requirements.
func SetIndexDBForTesting(db *dbsql.IndexDB) {
	indexDB = db
	// Reset the once to allow re-initialization in tests
	indexDBOnce = sync.Once{}
}

// calculateTotalComplexity sums up the complexity of all indexes.
// Returns the total complexity across all indexes.
func calculateTotalComplexity() uint {
	indexesMutex.RLock()
	defer indexesMutex.RUnlock()

	var totalComplexity uint = 0
	for _, idx := range indexes {
		idx.mu.RLock()
		// Only count complexity if index has been scanned at least once
		if idx.Complexity > 0 {
			totalComplexity += idx.Complexity
		}
		idx.mu.RUnlock()
	}
	return totalComplexity
}

// updateIndexDBCacheSize updates the shared index database cache size
// based on the total complexity of all indexes.
func updateIndexDBCacheSize() {
	if indexDB == nil {
		return
	}

	totalComplexity := calculateTotalComplexity()
	// Calculate cache size: complexity * 5MB
	cacheSizeMB := int(totalComplexity) * 2
	// Ensure size between 5MB and 100MB
	cacheSizeMB = utils.Clamp(cacheSizeMB, 2, 50)

	if err := indexDB.UpdateCacheSize(cacheSizeMB); err != nil {
		logger.Errorf("Failed to update index database cache size to %dMB: %v", cacheSizeMB, err)
	} else {
		logger.Debugf("Updated index database cache size to %dMB (total complexity: %d, capped at 25MB)", clampedCacheSizeMB, totalComplexity)
	}
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
		mock:            mock,
		Source:          *source,
		db:              indexDB, // Use shared database
		processedInodes: make(map[uint64]struct{}),
		FoundHardLinks:  make(map[string]uint64),
	}
	newIndex.ReducedIndex = ReducedIndex{
		Status:     "indexing",
		IdxName:    source.Name,
		Complexity: 0, // 0 = unknown until first scan completes
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

// indexDirectoryWithOptions wraps indexDirectory with actionConfig
func (idx *Index) indexDirectoryWithOptions(adjustedPath string, config actionConfig) error {
	return idx.indexDirectory(adjustedPath, config)
}

// Define a function to recursively index files and directories
func (idx *Index) indexDirectory(adjustedPath string, config actionConfig) error {
	// Normalize path to always have trailing slash
	adjustedPath = utils.AddTrailingSlashIfNotExists(adjustedPath)
	realPath := strings.TrimRight(idx.Path, "/") + adjustedPath
	// Open the directory
	dir, err := os.Open(realPath)
	if err != nil {
		// must have been deleted
		return err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}

	// check if excluded from indexing
	hidden := isHidden(dirInfo, idx.Path+adjustedPath)
	if idx.shouldSkip(dirInfo.IsDir(), hidden, adjustedPath, dirInfo.Name(), config) {
		return errors.ErrNotIndexed
	}

	// adjustedPath is already normalized with trailing slash
	combinedPath := adjustedPath
	// get whats currently in cache
	idx.mu.RLock()
	cacheDirItems := []iteminfo.ItemInfo{}
	modChange := false
	var cachedDir *iteminfo.FileInfo
	if idx.db != nil {
		// adjustedPath is already an index path (relative to source root)
		cachedDir, _ = idx.db.GetItem(idx.Name, adjustedPath)
	}
	if cachedDir != nil {
		modChange = dirInfo.ModTime().Unix() != cachedDir.ModTime.Unix()
		// adjustedPath is already an index path (relative to source root)
		if children, getErr := idx.db.GetDirectoryChildren(idx.Name, adjustedPath); getErr == nil {
			for _, child := range children {
				if child.Type == "directory" {
					cacheDirItems = append(cacheDirItems, child.ItemInfo)
				}
			}
		}
	}
	idx.mu.RUnlock()

	// If the directory has not been modified since the last index, skip expensive readdir
	// recursively check cached dirs for mod time changes as well
	if config.Recursive {
		if modChange {
			// Mark files as changed in the active scanner
			idx.markFilesChanged()
		} else if config.Quick {
			for _, item := range cacheDirItems {
				subConfig := actionConfig{
					Quick:     config.Quick,
					Recursive: true,
				}
				err = idx.indexDirectory(combinedPath+item.Name, subConfig)
				if err != nil && err != errors.ErrNotIndexed {
					logger.Errorf("error indexing directory %v : %v", combinedPath+item.Name, err)
				}
			}
			return nil
		}
	}
	dirFileInfo, err2 := idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, config)
	if err2 != nil {
		return err2
	}
	// Update the current directory metadata in the index
	idx.UpdateMetadata(dirFileInfo)
	return nil
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
		realSize, _ := idx.handleFile(dirInfo, adjustedPath, realPath, idx.isRoutineScan)
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
			if idx.wasIndexed && config.Recursive && idx.Config.ResolvedConditionals != nil {
				if _, exists := idx.Config.ResolvedConditionals.NeverWatchPaths[fullCombined]; exists {
					realDirInfo, exists := idx.GetMetadataInfo(dirPath, true)
					if exists {
						itemInfo.Size = realDirInfo.Size
					}
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
				err = idx.indexDirectory(dirPath, config)
				if err != nil {
					logger.Errorf("Failed to index directory %s: %v", dirPath, err)
					continue
				}
			}
			realDirInfo, exists := idx.GetMetadataInfo(dirPath, true)
			if exists {
				itemInfo.Size = realDirInfo.Size
				itemInfo.HasPreview = realDirInfo.HasPreview
			}
			totalSize += itemInfo.Size
			itemInfo.Type = "directory"
			dirInfos = append(dirInfos, *itemInfo)
			if config.IsRoutineScan {
				idx.NumDirs++
				idx.incrementScannerDirs()
			}
		} else {
			realFilePath := realPath + "/" + file.Name()
			size, shouldCountSize := idx.handleFile(file, fullCombined, realFilePath, idx.isRoutineScan)
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
				idx.NumFiles++
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

func (idx *Index) RecursiveUpdateDirSizes(childInfo *iteminfo.FileInfo, previousSize int64) {
	sizeDelta := childInfo.Size - previousSize
	if sizeDelta == 0 {
		return
	}
	idx.updateParentDirSizesBatched(childInfo.Path, sizeDelta)
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
	targetPath := opts.Path
	if !opts.IsDir {
		targetPath = idx.MakeIndexPath(filepath.Dir(targetPath), true)
	}

	previousInfo, previousExists := idx.GetMetadataInfo(targetPath, true)

	realPath, _, err := idx.GetRealPath(targetPath)
	if err != nil {
		return err
	}

	dirInfo, err := os.Stat(realPath)
	if err != nil {
		idx.DeleteMetadata(targetPath, true, false)
		return nil
	}

	needsRefresh := true
	if previousExists && !opts.Recursive {
		if dirInfo.ModTime().Unix() == previousInfo.ModTime.Unix() {
			needsRefresh = false
		}
	}

	var previousSize int64
	if previousExists {
		previousSize = previousInfo.Size
	}

	if needsRefresh {
		acquired := idx.tryAcquireScanMutex(100 * time.Millisecond)
		if !acquired {
			logger.Debugf("[REFRESH] Scanner is running, skipping index update for %s", targetPath)
			return nil
		}
		defer idx.scanMutex.Unlock()

		config := actionConfig{
			Quick:     true,
			Recursive: opts.Recursive,
		}
		err = idx.indexDirectoryWithOptions(targetPath, config)
		if err != nil {
			return err
		}

		newInfo, exists := idx.GetMetadataInfo(targetPath, true)
		if !exists {
			return nil
		}

		sizeDelta := newInfo.Size - previousSize
		if sizeDelta != 0 {
			idx.updateParentDirSizesBatched(targetPath, sizeDelta)
		}
	}

	return nil
}

// updateParentDirSizesBatched updates all parent directory sizes in a single batch operation.
// This queries all parent paths from the database, updates their sizes, and batch updates them.
// Optimized for SQLite - single query to get all parents, single transaction to update all sizes.
// No mutex needed - SQLite handles all locking internally for maximum concurrency.
func (idx *Index) updateParentDirSizesBatched(startPath string, sizeDelta int64) {
	if sizeDelta == 0 {
		return
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
		return
	}

	// Query all parent directories from database in a single query
	// SQLite handles locking - no mutex needed
	parentInfos, err := idx.db.GetItemsByPaths(idx.Name, parentPaths)
	if err != nil {
		logger.Errorf("[PARENT_SIZE] Failed to query parent directories for size update: %v", err)
		return
	}

	// Build map of path -> size delta for batch update
	// Only include paths that actually exist in the database
	pathSizeUpdates := make(map[string]int64)
	for _, path := range parentPaths {
		if _, exists := parentInfos[path]; exists {
			pathSizeUpdates[path] = sizeDelta
		}
	}

	if len(pathSizeUpdates) == 0 {
		return
	}

	// Batch update all parent sizes in a single transaction
	// SQLite handles locking - no mutex needed
	err = idx.db.BulkUpdateSizes(idx.Name, pathSizeUpdates)
	if err != nil {
		logger.Errorf("[PARENT_SIZE] Failed to batch update parent directory sizes: %v", err)
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
	switch status {
	case INDEXING:
		idx.runningScannerCount++
	case READY, UNAVAILABLE:
		idx.runningScannerCount = 0
	}
	idx.mu.Unlock()
	return idx.SendSourceUpdateEvent()
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

func (idx *Index) performPeriodicMaintenance() {
	if idx.db == nil {
		return
	}

	idx.mu.Lock()
	lastVacuum := idx.lastVacuumTime
	idx.mu.Unlock()

	if time.Since(lastVacuum) < 7*24*time.Hour {
		return
	}

	logger.Infof("[DB_MAINTENANCE] Starting periodic maintenance for index: %s", idx.Name)

	if err := idx.db.Vacuum(); err != nil {
		logger.Errorf("[DB_MAINTENANCE] Periodic maintenance failed for %s: %v", idx.Name, err)
		return
	}

	idx.mu.Lock()
	idx.lastVacuumTime = time.Now()
	idx.mu.Unlock()

	logger.Infof("[DB_MAINTENANCE] Periodic maintenance completed for index: %s", idx.Name)
}

func (idx *Index) tryAcquireScanMutex(timeout time.Duration) bool {
	lockChan := make(chan bool, 1)

	go func() {
		idx.scanMutex.Lock()
		select {
		case lockChan <- true:
		default:
			idx.scanMutex.Unlock()
		}
	}()

	select {
	case <-lockChan:
		return true
	case <-time.After(timeout):
		return false
	}
}
