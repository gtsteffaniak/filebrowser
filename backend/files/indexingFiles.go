package files

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

type Index struct {
	settings.Source
	Directories                map[string]*FileInfo
	NumDirs                    uint64
	NumFiles                   uint64
	NumDeleted                 uint64
	FilesChangedDuringIndexing bool
	currentSchedule            int
	assessment                 string
	indexingTime               int
	LastIndexed                time.Time
	SmartModifier              time.Duration
	mu                         sync.RWMutex
	scannerMu                  sync.Mutex
}

var (
	indexes      map[string]*Index
	indexesMutex sync.RWMutex
)

func Initialize(source settings.Source) {
	indexesMutex.RLock()
	defer indexesMutex.RUnlock()
	newIndex := Index{
		Source:      source,
		Directories: make(map[string]*FileInfo),
	}
	indexes = make(map[string]*Index)
	indexes[newIndex.Source.Name] = &newIndex
	if !newIndex.Source.Config.Disabled {
		time.Sleep(time.Second)
		log.Println("Initializing index and assessing file system complexity")
		newIndex.RunIndexing("/", false)
		go newIndex.setupIndexingScanners()
	} else {
		log.Println("Indexing disabled for source: ", newIndex.Source.Name)
	}
}

// Define a function to recursively index files and directories
func (idx *Index) indexDirectory(adjustedPath string, quick, recursive bool) error {
	realPath := strings.TrimRight(idx.Source.Path, "/") + adjustedPath
	// Open the directory
	dir, err := os.Open(realPath)
	if err != nil {
		idx.RemoveDirectory(adjustedPath) // Remove, must have been deleted
		return err
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return err
	}
	combinedPath := adjustedPath + "/"
	if adjustedPath == "/" {
		combinedPath = "/"
	}
	// get whats currently in cache
	idx.mu.RLock()
	cacheDirItems := []ItemInfo{}
	modChange := true // default to true
	cachedDir, exists := idx.Directories[adjustedPath]
	if exists && quick {
		modChange = dirInfo.ModTime() != cachedDir.ModTime
		cacheDirItems = cachedDir.Folders
	}
	idx.mu.RUnlock()

	// If the directory has not been modified since the last index, skip expensive readdir
	// recursively check cached dirs for mod time changes as well
	if !modChange && recursive {
		for _, item := range cacheDirItems {
			err = idx.indexDirectory(combinedPath+item.Name, quick, true)
			if err != nil {
				fmt.Printf("error indexing directory %v : %v", combinedPath+item.Name, err)
			}
		}
		return nil
	}

	if quick {
		idx.mu.Lock()
		idx.FilesChangedDuringIndexing = true
		idx.mu.Unlock()
	}

	// Read directory contents
	files, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var totalSize int64
	fileInfos := []ItemInfo{}
	dirInfos := []ItemInfo{}

	// Process each file and directory in the current directory
	for _, file := range files {
		fullCombined := combinedPath + file.Name()
		if idx.shouldSkip(file.IsDir(), isHidden(file, ""), fullCombined) {
			continue
		}
		itemInfo := &ItemInfo{
			Name:    file.Name(),
			ModTime: file.ModTime(),
		}
		if file.IsDir() {
			dirPath := combinedPath + file.Name()
			if recursive {
				// Recursively index the subdirectory
				err = idx.indexDirectory(dirPath, quick, recursive)
				if err != nil {
					log.Printf("Failed to index directory %s: %v", dirPath, err)
					continue
				}
			}
			realDirInfo, exists := idx.GetMetadataInfo(dirPath, true)
			if exists {
				itemInfo.Size = realDirInfo.Size
			}
			totalSize += itemInfo.Size
			itemInfo.Type = "directory"
			dirInfos = append(dirInfos, *itemInfo)
			idx.NumDirs++
		} else {
			itemInfo.DetectType(fullCombined, false)
			itemInfo.Size = file.Size()
			fileInfos = append(fileInfos, *itemInfo)
			totalSize += itemInfo.Size
			idx.NumFiles++
		}
	}
	if totalSize == 0 && idx.Source.Config.IgnoreZeroSizeFolders {
		return nil
	}
	// Create FileInfo for the current directory
	dirFileInfo := &FileInfo{
		Path:    adjustedPath,
		Files:   fileInfos,
		Folders: dirInfos,
	}
	dirFileInfo.ItemInfo = ItemInfo{
		Name:    dirInfo.Name(),
		Type:    "directory",
		Size:    totalSize,
		ModTime: dirInfo.ModTime(),
	}

	dirFileInfo.SortItems()

	// Update the current directory metadata in the index
	idx.UpdateMetadata(dirFileInfo)

	return nil
}

func (idx *Index) makeIndexPath(subPath string) string {
	subPath = strings.ReplaceAll(subPath, "\\", "/")
	if strings.HasPrefix(subPath, "./") {
		subPath = strings.TrimPrefix(subPath, ".")
	}
	if idx.Source.Path == subPath || subPath == "." {
		return "/"
	}
	// clean path
	subPath = strings.TrimSuffix(subPath, "/")
	// remove index prefix
	adjustedPath := strings.TrimPrefix(subPath, idx.Source.Path)
	// remove trailing slash
	adjustedPath = strings.TrimSuffix(adjustedPath, "/")
	if !strings.HasPrefix(adjustedPath, "/") {
		adjustedPath = "/" + adjustedPath
	}
	return adjustedPath
}

func (idx *Index) recursiveUpdateDirSizes(childInfo *FileInfo, previousSize int64) {
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

func (idx *Index) RefreshFileInfo(opts FileOptions) error {
	refreshOptions := FileOptions{
		Path:  opts.Path,
		IsDir: opts.IsDir,
	}

	if !refreshOptions.IsDir {
		refreshOptions.Path = idx.makeIndexPath(filepath.Dir(refreshOptions.Path))
		refreshOptions.IsDir = true
	} else {
		refreshOptions.Path = idx.makeIndexPath(refreshOptions.Path)
	}
	err := idx.indexDirectory(refreshOptions.Path, false, false)
	if err != nil {
		return fmt.Errorf("file/folder does not exist to refresh data: %s", refreshOptions.Path)
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

func isHidden(file os.FileInfo, realpath string) bool {
	return file.Name()[0] == '.'
}

func (idx *Index) shouldSkip(isDir bool, isHidden bool, fullCombined string) bool {
	// check inclusions first
	if isDir && len(idx.Source.Config.Include.Folders) > 0 {
		if !slices.Contains(idx.Source.Config.Include.Folders, fullCombined) {
			return true
		}
	}
	if !isDir && len(idx.Source.Config.Include.Files) > 0 {
		if !slices.Contains(idx.Source.Config.Include.Files, fullCombined) {
			return true
		}
	}

	// check exclusions
	if isDir && slices.Contains(idx.Source.Config.Exclude.Folders, fullCombined) {
		return true
	}
	if !isDir && slices.Contains(idx.Source.Config.Exclude.Files, fullCombined) {
		return true
	}
	if idx.Source.Config.IgnoreHidden && isHidden {
		return true
	}
	return false
}
