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

var RealPathCache = cache.NewCache(48*time.Hour, 72*time.Hour)

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
var omitList = []string{"$RECYCLE.BIN", "System Volume Information", "@eaDir"}

func init() {
	indexes = make(map[string]*Index)
}

func Initialize(source settings.Source, mock bool) {
	indexesMutex.Lock()
	newIndex := Index{
		mock:              mock,
		Source:            source,
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

// Define a function to recursively index files and directories
func (idx *Index) indexDirectory(adjustedPath string, quick, recursive bool) error {
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
	if recursive {
		// Prevent race conditions if scanning becomes concurrent in the future.
		idx.mu.Lock()
		idx.DirectoriesLedger[adjustedPath] = struct{}{}
		idx.mu.Unlock()
	}
	combinedPath := adjustedPath + "/"
	if adjustedPath == "/" {
		combinedPath = "/"
	}
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
	if recursive {
		if modChange {
			idx.mu.Lock()
			idx.FilesChangedDuringIndexing = true
			idx.mu.Unlock()
		} else if quick {
			for _, item := range cacheDirItems {
				err = idx.indexDirectory(combinedPath+item.Name, quick, true)
				if err != nil && err != errors.ErrNotIndexed {
					logger.Errorf("error indexing directory %v : %v", combinedPath+item.Name, err)
				}
			}
			return nil
		}
	}
	dirFileInfo, err2 := idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, quick, recursive, true)
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
	combinedPath := adjustedPath + "/"
	if adjustedPath == "/" {
		combinedPath = "/"
	}
	var response *iteminfo.FileInfo
	response, err = idx.GetDirInfo(dir, dirInfo, realPath, adjustedPath, combinedPath, false, false, false)
	if err != nil {
		return nil, err
	}
	if !isDir {
		baseName := filepath.Base(originalPath)
		idx.MakeIndexPath(realPath)
		found := false
		for _, item := range response.Files {
			if item.Name == baseName {
				response = &iteminfo.FileInfo{
					Path:     adjustedPath + "/" + item.Name,
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

func (idx *Index) GetDirInfo(dirInfo *os.File, stat os.FileInfo, realPath, adjustedPath, combinedPath string, quick, recursive, checkSkip bool) (*iteminfo.FileInfo, error) {
	// Read directory contents
	files, err := dirInfo.Readdir(-1)
	if err != nil {
		return nil, err
	}
	var totalSize int64
	fileInfos := []iteminfo.ItemInfo{}
	dirInfos := []iteminfo.ItemInfo{}
	hasPreview := false
	if !recursive {
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
		if checkSkip && idx.shouldSkip(isDir, hidden, fullCombined, baseName) {
			continue
		}
		itemInfo := &iteminfo.ItemInfo{
			Name:    file.Name(),
			ModTime: file.ModTime(),
			Hidden:  hidden,
		}

		if isDir {
			dirPath := combinedPath + file.Name()
			if idx.hasIndex && recursive && len(idx.Config.NeverWatchPaths) > 0 {
				if slices.Contains(idx.Config.NeverWatchPaths, fullCombined) {
					realDirInfo, exists := idx.GetMetadataInfo(dirPath, true)
					if exists {
						itemInfo.Size = realDirInfo.Size
					}
					continue
				}
			}
			// skip non-indexable dirs.
			if slices.Contains(omitList, file.Name()) {
				continue
			}

			if recursive {
				// clear for garbage collection
				file = nil
				// Recursively index the subdirectory
				err = idx.indexDirectory(dirPath, quick, recursive)
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
			if recursive {
				idx.NumDirs++
			}
		} else {
			size, shouldCountSize := idx.handleFile(file, fullCombined)
			itemInfo.DetectType(realPath+"/"+file.Name(), false)
			simpleType := strings.Split(itemInfo.Type, "/")[0]
			if simpleType == "audio" {
				if recursive && !quick && idx.Config.IndexAlbumArt {
					itemInfo.HasPreview = iteminfo.HasAlbumArt(realPath+"/"+file.Name(), filepath.Ext(file.Name()))
				}
				if !recursive && !itemInfo.HasPreview {
					info, exists := idx.GetReducedMetadata(fullCombined, false)
					if exists {
						itemInfo.HasPreview = info.HasPreview
					}
				}
			}
			ext := strings.ToLower(filepath.Ext(file.Name()))
			switch ext {
			case ".jpg", ".jpeg", ".png", ".bmp", ".tiff":
				itemInfo.HasPreview = true
			case ".heic", ".heif":
				if settings.Config.Integrations.Media.FfmpegPath != "" {
					itemInfo.HasPreview = true
				}
			}
			if simpleType == "image" {
				itemInfo.HasPreview = true
			}
			if settings.Config.Integrations.OnlyOffice.Secret != "" && iteminfo.IsOnlyOffice(file.Name()) {
				itemInfo.HasPreview = true
			}
			if settings.Config.Integrations.Media.FfmpegPath != "" && simpleType == "video" {
				itemInfo.HasPreview = true
			}
			if settings.Config.Integrations.Media.FfmpegPath != "" && strings.HasPrefix(itemInfo.Type, "video") && itemInfo.HasPreview {
				itemInfo.HasPreview = true
			}
			if iteminfo.HasDocConvertableExtension(itemInfo.Name, itemInfo.Type) {
				itemInfo.HasPreview = true
			}

			itemInfo.Size = int64(size)
			fileInfos = append(fileInfos, *itemInfo)
			if shouldCountSize {
				totalSize += itemInfo.Size
			}
			if recursive {
				idx.NumFiles++
			}
			if itemInfo.HasPreview {
				hasPreview = true
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

	// Create FileInfo for the current directory
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

// input should be non-index path.
func (idx *Index) MakeIndexPath(subPath string) string {
	if strings.HasPrefix(subPath, "./") {
		subPath = strings.TrimPrefix(subPath, ".")
	}
	if idx.Path == subPath || subPath == "." {
		return "/"
	}
	// clean path
	subPath = strings.TrimSuffix(subPath, "/")
	adjustedPath := strings.TrimPrefix(subPath, idx.Path)
	// remove index prefix
	adjustedPath = strings.ReplaceAll(adjustedPath, "\\", "/")
	// remove trailing slash
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	if !strings.HasPrefix(adjustedPath, "/") {
		adjustedPath = "/" + adjustedPath
	}
	return adjustedPath
}

func (idx *Index) recursiveUpdateDirSizes(childInfo *iteminfo.FileInfo, previousSize int64) {
	parentDir := utils.GetParentDirectoryPath(childInfo.Path)
	parentInfo, exists := idx.GetMetadataInfo(parentDir, true)
	if !exists || parentDir == "" {
		return
	}
	newSize := parentInfo.Size - previousSize + childInfo.Size
	parentInfo.Size += newSize
	idx.UpdateMetadata(parentInfo)
	idx.recursiveUpdateDirSizes(parentInfo, newSize)
}

func (idx *Index) GetRealPath(relativePath ...string) (string, bool, error) {
	combined := append([]string{idx.Path}, relativePath...)
	joinedPath := filepath.Join(combined...)
	isDir, _ := RealPathCache.Get(joinedPath + ":isdir").(bool)
	cached, ok := RealPathCache.Get(joinedPath).(string)
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
		RealPathCache.Set(joinedPath+":isdir", isDir)
	}
	return realPath, isDir, err
}

func (idx *Index) RefreshFileInfo(opts iteminfo.FileOptions) error {
	refreshOptions := iteminfo.FileOptions{
		Path:  opts.Path,
		IsDir: opts.IsDir,
	}
	if !refreshOptions.IsDir {
		refreshOptions.Path = idx.MakeIndexPath(filepath.Dir(refreshOptions.Path))
		refreshOptions.IsDir = true
	}
	err := idx.indexDirectory(refreshOptions.Path, false, false)
	if err != nil {
		return err
	}
	file, exists := idx.GetMetadataInfo(refreshOptions.Path, true)
	if !exists {
		return fmt.Errorf("file/folder does not exist in metadata: %s", refreshOptions.Path)
	}

	current, firstExisted := idx.GetMetadataInfo(refreshOptions.Path, true)
	refreshParentInfo := firstExisted && current.Size != file.Size
	//utils.PrintStructFields(*file)
	result := idx.UpdateMetadata(file)
	if !result {
		return fmt.Errorf("file/folder does not exist in metadata: %s", refreshOptions.Path)
	}
	if !exists {
		return nil
	}
	if refreshParentInfo {
		idx.recursiveUpdateDirSizes(file, current.Size)
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
