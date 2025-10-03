package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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
	Quick      bool // whether to perform a quick scan (skip unchanged directories)
	Recursive  bool // whether to recursively index subdirectories
	ForceCheck bool // whether to check indexing skip rules.
}

// NewactionConfig creates a new actionConfig with common presets
func NewactionConfig() *actionConfig {
	return &actionConfig{
		Quick:     false,
		Recursive: true,
	}
}

// reduced index is json exposed to the client
type ReducedIndex struct {
	IdxName         string      `json:"name"`
	DiskUsed        uint64      `json:"used"`
	DiskTotal       uint64      `json:"total"`
	Status          IndexStatus `json:"status"`
	NumDirs         uint64      `json:"numDirs"`
	NumFiles        uint64      `json:"numFiles"`
	NumDeleted      uint64      `json:"numDeleted"`
	LastIndexed     time.Time   `json:"-"`
	LastIndexedUnix int64       `json:"lastIndexedUnixTime"`
	QuickScanTime   int         `json:"quickScanDurationSeconds"`
	FullScanTime    int         `json:"fullScanDurationSeconds"`
	Assessment      string      `json:"assessment"`
}
type Index struct {
	ReducedIndex
	CurrentSchedule            int `json:"-"`
	settings.Source            `json:"-"`
	Directories                map[string]*iteminfo.FileInfo `json:"-"`
	DirectoriesLedger          map[string]struct{}           `json:"-"`
	runningScannerCount        int                           `json:"-"`
	SmartModifier              time.Duration                 `json:"-"`
	FilesChangedDuringIndexing bool                          `json:"-"`
	mock                       bool
	mu                         sync.RWMutex
	hasIndex                   bool
	FoundHardLinks             map[string]uint64   `json:"-"` // hardlink path -> size
	processedInodes            map[uint64]struct{} `json:"-"`
	totalSize                  uint64              `json:"-"`
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
		Assessment: "unknown",
	}
	indexes[newIndex.Name] = &newIndex
	indexesMutex.Unlock()
	if !newIndex.Config.DisableIndexing {
		time.Sleep(time.Second)
		logger.Infof("initializing index: [%v]", newIndex.Name)
		newIndex.RunIndexing("/", false)
		go newIndex.setupIndexingScanners()
	} else {
		newIndex.Status = "ready"
		logger.Debug("indexing disabled for source: " + newIndex.Name)
	}
}

// indexDirectoryWithOptions wraps indexDirectory with actionConfig
func (idx *Index) indexDirectoryWithOptions(adjustedPath string, config *actionConfig) error {
	return idx.indexDirectory(adjustedPath, config)
}

// Define a function to recursively index files and directories
func (idx *Index) indexDirectory(adjustedPath string, config *actionConfig) error {
	// Normalize path to always have trailing slash (except for root which is just "/")
	if adjustedPath != "/" {
		adjustedPath = strings.TrimSuffix(adjustedPath, "/") + "/"
	}
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
	if idx.shouldSkip(dirInfo.IsDir(), hidden, adjustedPath, dirInfo.Name()) {
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
			idx.mu.Lock()
			idx.FilesChangedDuringIndexing = true
			idx.mu.Unlock()
		} else if config.Quick {
			for _, item := range cacheDirItems {
				subConfig := &actionConfig{
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
		fileInfo := iteminfo.FileInfo{
			Path: adjustedPath,
			ItemInfo: iteminfo.ItemInfo{
				Name:    filepath.Base(originalPath),
				Size:    dirInfo.Size(),
				ModTime: dirInfo.ModTime(),
			},
		}
		fileInfo.DetectType(realPath, false)
		return &fileInfo, nil
	}

	// Normalize directory path to always have trailing slash
	if adjustedPath != "/" {
		adjustedPath = strings.TrimSuffix(adjustedPath, "/") + "/"
	}
	// adjustedPath is already normalized with trailing slash
	combinedPath := adjustedPath
	var response *iteminfo.FileInfo
	response, err = idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, &actionConfig{
		Quick:      false,
		Recursive:  false,
		ForceCheck: true,
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
				// Clean path to remove trailing slashes before joining
				filePath := strings.TrimSuffix(adjustedPath, "/") + "/" + item.Name
				response = &iteminfo.FileInfo{
					Path:     filePath,
					ItemInfo: item,
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

func (idx *Index) GetDirInfo(dirInfo *os.File, stat os.FileInfo, realPath, adjustedPath, combinedPath string, config *actionConfig) (*iteminfo.FileInfo, error) {
	// Ensure combinedPath has exactly one trailing slash to prevent double slashes in subdirectory paths
	combinedPath = strings.TrimRight(combinedPath, "/") + "/"
	// Read directory contents
	files, err := dirInfo.Readdir(-1)
	if err != nil {
		return nil, err
	}
	var totalSize int64
	fileInfos := []iteminfo.ItemInfo{}
	dirInfos := []iteminfo.ItemInfo{}
	hasPreview := false
	if !config.Recursive {
		realDirInfo, exists := idx.GetMetadataInfo(adjustedPath, true)
		if exists {
			hasPreview = realDirInfo.HasPreview
		}
	}

	// Process each file and directory in the current directory
	for _, file := range files {
		hidden := isHidden(file, idx.Path+combinedPath)
		isDir := iteminfo.IsDirectory(file)
		baseName := file.Name()
		fullCombined := combinedPath + baseName
		if adjustedPath == "/" {
			if !idx.shouldInclude(isDir, combinedPath, file.Name()) {
				continue
			}
		}
		if !config.ForceCheck && idx.shouldSkip(isDir, hidden, fullCombined, baseName) {
			continue
		}
		itemInfo := &iteminfo.ItemInfo{
			Name:    file.Name(),
			ModTime: file.ModTime(),
			Hidden:  hidden,
		}

		if isDir {
			dirPath := combinedPath + file.Name()
			if idx.hasIndex && config.Recursive && len(idx.Config.NeverWatchPaths) > 0 {
				if slices.Contains(idx.Config.NeverWatchPaths, fullCombined) {
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
				// Recursively index the subdirectory
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
			if config.Recursive {
				idx.NumDirs++
			}
		} else {
			size, shouldCountSize := idx.handleFile(file, fullCombined)
			itemInfo.DetectType(realPath+"/"+file.Name(), false)
			simpleType := strings.Split(itemInfo.Type, "/")[0]
			if simpleType == "audio" {
				// Check if this file is new or changed by comparing with previous metadata
				shouldCheckAlbumArt := false
				previousInfo, exists := idx.GetReducedMetadata(fullCombined, false)
				if !exists {
					// File is new - check album art
					shouldCheckAlbumArt = true
				} else if previousInfo.ModTime != file.ModTime() {
					// File has been modified - check album art
					shouldCheckAlbumArt = true
				} else {
					// File unchanged - use cached album art info
					itemInfo.HasPreview = previousInfo.HasPreview
				}

				if shouldCheckAlbumArt {
					itemInfo.HasPreview = iteminfo.HasAlbumArt(realPath+"/"+file.Name(), filepath.Ext(file.Name()))
				}
			}
			ext := strings.ToLower(filepath.Ext(file.Name()))
			extWithoutPeriod := strings.TrimPrefix(ext, ".")
			if simpleType == "image" {
				itemInfo.HasPreview = true
			}
			switch extWithoutPeriod {
			case "heic", "heif":
				if settings.CanConvertImage(extWithoutPeriod) {
					itemInfo.HasPreview = true
				}
			}
			if simpleType == "video" && settings.CanConvertVideo(extWithoutPeriod) {
				itemInfo.HasPreview = true
			}
			itemInfo.Size = int64(size)

			// Update parent folder preview status for images, videos, and audio with album art
			// Use shared function to determine if this file type should bubble up to folder preview
			if itemInfo.HasPreview && iteminfo.ShouldBubbleUpToFolderPreview(*itemInfo) {
				hasPreview = true
			}

			// Set HasPreview for office docs and PDFs
			// These files are previewable but DON'T set parent folder preview
			if settings.Config.Integrations.OnlyOffice.Secret != "" && iteminfo.IsOnlyOffice(file.Name()) {
				itemInfo.HasPreview = true
			}
			if iteminfo.HasDocConvertableExtension(itemInfo.Name, itemInfo.Type) {
				itemInfo.HasPreview = true
			}

			fileInfos = append(fileInfos, *itemInfo)
			if shouldCountSize {
				totalSize += itemInfo.Size
			}
			if config.Recursive {
				idx.NumFiles++
			}
		}
	}

	if totalSize == 0 && idx.Config.Exclude.ZeroSizeFolders {
		return nil, errors.ErrNotIndexed
	}

	if adjustedPath == "/" {
		idx.mu.Lock()
		idx.DiskUsed = uint64(totalSize)
		idx.mu.Unlock()
	}

	// Create FileInfo for the current directory (adjustedPath is already normalized with trailing slash)
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

	// Metadata will be updated by the caller (indexDirectory or GetFsDirInfo)
	return dirFileInfo, nil
}

func (idx *Index) recursiveUpdateDirSizes(childInfo *iteminfo.FileInfo, previousSize int64) {
	parentDir := utils.GetParentDirectoryPath(childInfo.Path)

	parentInfo, exists := idx.GetMetadataInfo(parentDir, true)
	if !exists || parentDir == "" {
		return
	}

	// Calculate size delta and update parent
	previousParentSize := parentInfo.Size
	sizeDelta := childInfo.Size - previousSize
	parentInfo.Size = previousParentSize + sizeDelta

	idx.UpdateMetadata(parentInfo)

	// Recursively update grandparents
	idx.recursiveUpdateDirSizes(parentInfo, previousParentSize)
}

func (idx *Index) GetRealPath(relativePath ...string) (string, bool, error) {
	combined := append([]string{idx.Path}, relativePath...)
	joinedPath := filepath.Join(combined...)
	isDir, _ := IsDirCache.Get(joinedPath + ":isdir")
	cached, ok := RealPathCache.Get(joinedPath)
	if ok && cached != "" {
		return cached, isDir, nil
	}
	// Convert relative path to absolute path
	absolutePath, err := filepath.Abs(joinedPath)
	if err != nil {
		return absolutePath, false, fmt.Errorf("could not get real path: %v, %s", joinedPath, err)
	}
	// Resolve symlinks and get the real path
	realPath, isDir, err := iteminfo.ResolveSymlinks(absolutePath)
	if err == nil {
		RealPathCache.Set(joinedPath, realPath)
		IsDirCache.Set(joinedPath+":isdir", isDir)
	}
	return realPath, isDir, err
}

func (idx *Index) RefreshFileInfo(opts utils.FileOptions) error {
	config := &actionConfig{
		Quick:     false,
		Recursive: opts.Recursive,
	}

	targetPath := opts.Path
	if !opts.IsDir {
		targetPath = idx.MakeIndexPath(filepath.Dir(targetPath))
	}

	// Get PREVIOUS metadata BEFORE indexing
	previousInfo, previousExists := idx.GetMetadataInfo(targetPath, true)
	var previousSize int64
	if previousExists {
		previousSize = previousInfo.Size
	}

	// Re-index the directory
	err := idx.indexDirectoryWithOptions(targetPath, config)
	if err != nil {
		return err
	}

	// Get the NEW metadata after indexing
	newInfo, exists := idx.GetMetadataInfo(targetPath, true)
	if !exists {
		return fmt.Errorf("file/folder does not exist in metadata: %s", targetPath)
	}

	// If size changed, propagate to parents
	if previousSize != newInfo.Size {
		idx.recursiveUpdateDirSizes(newInfo, previousSize)
	}

	return nil
}

func isHidden(file os.FileInfo, srcPath string) bool {
	// Check if the file starts with a dot (common on Unix systems)
	if file.Name()[0] == '.' {
		return true
	}

	if runtime.GOOS == "windows" {
		return CheckWindowsHidden(filepath.Join(srcPath, file.Name()))
	}

	// Default behavior for non-Windows systems
	return false
}

func (idx *Index) shouldInclude(isDir bool, fullCombined, baseName string) bool {
	rules := idx.Config.Include
	hasRules := false
	if len(rules.RootFolders) > 0 {
		hasRules = true
		for _, p := range rules.RootFolders {
			if strings.HasPrefix(fullCombined, p) {
				return true
			}
		}
	}
	if len(rules.RootFiles) > 0 {
		hasRules = true
		for _, p := range rules.RootFiles {
			if strings.HasPrefix(fullCombined, p) {
				return true
			}
		}
	}
	if !hasRules {
		return true
	}
	return false
}

func (idx *Index) shouldSkip(isDir bool, isHidden bool, fullCombined, baseName string) bool {
	if idx.Config.DisableIndexing {
		return true
	}

	rules := idx.Config.Exclude

	if isDir {
		if len(rules.FolderPaths) > 0 {
			for _, p := range rules.FolderPaths {
				if strings.HasPrefix(fullCombined, p) {
					return true
				}
			}
		}
		if len(rules.FolderNames) > 0 && slices.Contains(rules.FolderNames, baseName) {
			return true
		}
		if len(rules.FolderEndsWith) > 0 {
			for _, end := range rules.FolderEndsWith {
				if strings.HasSuffix(baseName, end) {
					return true
				}
			}
		}
		if len(rules.FolderStartsWith) > 0 {
			for _, start := range rules.FolderStartsWith {
				if strings.HasPrefix(baseName, start) {
					return true
				}
			}
		}
	} else {
		if len(rules.FilePaths) > 0 {
			for _, p := range rules.FilePaths {
				if strings.HasPrefix(fullCombined, p) {
					return true
				}
			}
		}
		if len(rules.FileNames) > 0 && slices.Contains(rules.FileNames, baseName) {
			return true
		}
		if len(rules.FileEndsWith) > 0 {
			for _, end := range rules.FileEndsWith {
				if strings.HasSuffix(baseName, end) {
					return true
				}
			}
		}
		if len(rules.FileStartsWith) > 0 {
			for _, start := range rules.FileStartsWith {
				if strings.HasPrefix(baseName, start) {
					return true
				}
			}
		}
		// Exclude if parent directory matches FolderPaths
		if len(rules.FolderPaths) > 0 {
			parent := filepath.Dir(fullCombined)
			for _, p := range rules.FolderPaths {
				if strings.HasPrefix(parent, p) {
					return true
				}
			}
		}
	}

	if rules.Hidden && isHidden {
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

func (idx *Index) SetStatus(status IndexStatus) {
	idx.mu.Lock()
	idx.Status = status
	switch status {
	case INDEXING:
		idx.runningScannerCount++
	case READY, UNAVAILABLE:
		idx.runningScannerCount = 0
	}
	idx.mu.Unlock()
	idx.SendSourceUpdateEvent()
}

func (idx *Index) handleFile(file os.FileInfo, fullCombined string) (size uint64, shouldCountSize bool) {
	var realSize uint64
	var nlink uint64 = 1
	var ino uint64 = 0
	canUseSyscall := false

	if sys := file.Sys(); sys != nil {
		realSize, nlink, ino, canUseSyscall = getFileDetails(sys)
	}

	if !canUseSyscall {
		// Fallback for non-unix systems or if syscall info is unavailable
		realSize = uint64(file.Size())
	}

	if nlink > 1 {
		// It's a hard link
		idx.mu.Lock()
		defer idx.mu.Unlock()
		if _, exists := idx.processedInodes[ino]; exists {
			// Already seen, don't count towards global total, or directory total.
			return realSize, false
		}
		// First time seeing this inode.
		idx.processedInodes[ino] = struct{}{}
		idx.FoundHardLinks[fullCombined] = realSize
		idx.totalSize += realSize
		return realSize, true // Count size for directory total.
	}

	// It's a regular file.
	idx.mu.Lock()
	idx.totalSize += realSize
	idx.mu.Unlock()
	return realSize, true // Count size.
}

// input should be non-index path.
func (idx *Index) MakeIndexPath(path string) string {
	if path == "." || strings.HasPrefix(path, "./") {
		path = strings.TrimPrefix(path, ".")
	}
	path = strings.TrimPrefix(path, idx.Path)
	path = idx.MakeIndexPathPlatform(path)
	path = strings.TrimSuffix(path, "/") + "/"
	return path
}
