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
	Directories       map[string]*iteminfo.FileInfo `json:"-"`
	DirectoriesLedger map[string]struct{}           `json:"-"`
	FoundHardLinks    map[string]uint64             `json:"-"` // hardlink path -> size
	processedInodes   map[uint64]struct{}           `json:"-"`
	totalSize         uint64                        `json:"-"`
	previousTotalSize uint64                        `json:"-"` // Track previous totalSize for change detection

	// Scanner management (new multi-scanner system)
	scanners             map[string]*Scanner `json:"-"` // path -> scanner
	scanMutex            sync.Mutex          `json:"-"` // Global scan mutex - only one scanner runs at a time
	activeScannerPath    string              `json:"-"` // Which scanner is currently running (for logging/status)
	runningScannerCount  int                 `json:"-"` // Tracks active scanners
	lastRootScanTime     time.Time           `json:"-"` // Last time root scanner completed - child scanners wait for this
	initialScanStartTime time.Time           `json:"-"` // When initial multi-scanner indexing started
	hasLoggedInitialScan bool                `json:"-"` // Whether we've logged the first complete round

	// Control
	mock       bool
	mu         sync.RWMutex
	wasIndexed bool
}

var (
	indexes      map[string]*Index
	indexesMutex sync.RWMutex
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

func Initialize(source *settings.Source, mock bool) {
	indexesMutex.Lock()
	newIndex := Index{
		mock:              mock,
		Source:            *source,
		Directories:       make(map[string]*iteminfo.FileInfo),
		DirectoriesLedger: make(map[string]struct{}),
		processedInodes:   make(map[uint64]struct{}),
		FoundHardLinks:    make(map[string]uint64),
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

	// if indexing, mark the directory as valid and indexed.
	if config.Recursive {
		// Prevent race conditions if scanning becomes concurrent in the future.
		idx.mu.Lock()
		idx.DirectoriesLedger[adjustedPath] = struct{}{}
		idx.mu.Unlock()
	}
	// adjustedPath is already normalized with trailing slash
	combinedPath := adjustedPath
	// get whats currently in cache
	idx.mu.RLock()
	cacheDirItems := []iteminfo.ItemInfo{}
	modChange := false
	cachedDir, exists := idx.Directories[adjustedPath]
	if exists {
		modChange = dirInfo.ModTime() != cachedDir.ModTime
		cacheDirItems = cachedDir.Folders
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

func (idx *Index) GetFsInfo(adjustedPath string, followSymlinks bool) (*iteminfo.FileInfo, error) {
	realPath, isDir, err := idx.GetRealPath(followSymlinks, adjustedPath)
	if err != nil {
		return nil, err
	}
	originalPath := realPath

	// If not following symlinks, check if it's a symlink first
	if !followSymlinks {
		symlinkInfo, err := os.Lstat(realPath)
		if err != nil {
			return nil, err
		}

		// If it's a symlink, return info about the symlink itself (not the target)
		if symlinkInfo.Mode()&os.ModeSymlink != 0 {
			realSize, _ := idx.handleFile(symlinkInfo, adjustedPath, realPath)
			size := int64(realSize)
			fileInfo := iteminfo.FileInfo{
				Path: adjustedPath,
				ItemInfo: iteminfo.ItemInfo{
					Name:    filepath.Base(strings.TrimSuffix(adjustedPath, "/")),
					Size:    size,
					ModTime: symlinkInfo.ModTime(),
					Type:    "symlink",
				},
			}
			return &fileInfo, nil
		}
	}

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
		realSize, _ := idx.handleFile(dirInfo, adjustedPath, realPath)
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
		idx.MakeIndexPath(realPath)
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
			size, shouldCountSize := idx.handleFile(file, fullCombined, realFilePath)
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
	if totalSize == 0 && idx.Config.Conditionals.ZeroSizeFolders && combinedPath != "/" {
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

// RecursiveUpdateDirSizes updates parent directory sizes recursively up the tree
func (idx *Index) RecursiveUpdateDirSizes(childInfo *iteminfo.FileInfo, previousSize int64) {
	parentDir := utils.GetParentDirectoryPath(childInfo.Path)
	parentInfo, exists := idx.GetMetadataInfo(parentDir, true)
	if !exists || parentDir == "" {
		return
	}
	previousParentSize := parentInfo.Size
	sizeDelta := childInfo.Size - previousSize
	parentInfo.Size = previousParentSize + sizeDelta
	idx.UpdateMetadata(parentInfo)
	idx.RecursiveUpdateDirSizes(parentInfo, previousParentSize)
}

func (idx *Index) GetRealPath(followSymlinks bool, relativePath ...string) (string, bool, error) {
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

	if !followSymlinks {
		// Use Lstat to check if it's a symlink without following it
		info, err := os.Lstat(absolutePath)
		if err != nil {
			return absolutePath, false, fmt.Errorf("could not stat path: %s, %v", absolutePath, err)
		}

		// For symlinks, we still need to know if the target is a directory
		isDir := false
		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink - check if target is a directory
			targetInfo, err := os.Stat(absolutePath)
			if err == nil {
				isDir = iteminfo.IsDirectory(targetInfo)
			}
			// If we can't stat the target, assume it's a file (broken symlink or file symlink)
		} else {
			isDir = iteminfo.IsDirectory(info)
		}

		return absolutePath, isDir, nil
	}

	// Follow symlinks
	realPath, isDir, err := iteminfo.ResolveSymlinks(absolutePath)
	if err == nil {
		RealPathCache.Set(joinedPath, realPath)
		IsDirCache.Set(joinedPath+":isdir", isDir)
	}
	return realPath, isDir, err
}

func (idx *Index) RefreshFileInfo(opts utils.FileOptions) error {
	config := actionConfig{
		Quick:     false,
		Recursive: opts.Recursive,
	}
	targetPath := opts.Path
	if !opts.IsDir {
		targetPath = idx.MakeIndexPath(filepath.Dir(targetPath))
	}
	previousInfo, previousExists := idx.GetMetadataInfo(targetPath, true)
	var previousSize int64
	if previousExists {
		previousSize = previousInfo.Size
	}
	err := idx.indexDirectoryWithOptions(targetPath, config)
	if err != nil {
		return err
	}
	newInfo, exists := idx.GetMetadataInfo(targetPath, true)
	if !exists {
		return fmt.Errorf("file/folder does not exist in metadata: %s", targetPath)
	}
	if previousSize != newInfo.Size {
		idx.RecursiveUpdateDirSizes(newInfo, previousSize)
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
func (idx *Index) MakeIndexPath(path string) string {
	if path == "." || strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, ".")
	}
	path = strings.TrimPrefix(path, idx.Path)
	path = idx.MakeIndexPathPlatform(path)
	path = utils.AddTrailingSlashIfNotExists(path)
	return path
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
