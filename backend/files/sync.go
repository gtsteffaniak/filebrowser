package files

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (si *Index) UpdateFileMetadata(adjustedPath string, info *FileInfo) bool {
	checkDir := si.makeIndexPath(adjustedPath)
	if info.Type != "directory" {
		checkDir = si.makeIndexPath(filepath.Dir(adjustedPath))
	}
	si.mu.Lock()
	defer si.mu.Unlock()
	_, exists := si.Directories[checkDir]
	if !exists {
		info.CacheTime = time.Now()
		si.Directories[checkDir] = info
		return true
	}
	si.Directories[checkDir] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetMetadataInfo(target string, isDir bool) (FileInfo, bool) {
	checkDir := si.makeIndexPath(target)
	if !isDir {
		checkDir = si.makeIndexPath(filepath.Dir(target))
	}
	fmt.Println("checkDir: ", checkDir)
	si.mu.RLock()
	dir, exists := si.Directories[checkDir]
	si.mu.RUnlock()
	if !exists {
		return FileInfo{}, exists
	}
	if !isDir {
		baseName := filepath.Base(target)
		fileInfo, ok := dir.Files[baseName]
		if !ok {
			fmt.Println("file not found in meta", baseName)
			return FileInfo{}, false
		}

		fmt.Println("file found in meta", fileInfo.Path)
		return *fileInfo, ok
	}
	cleanedItems := []ReducedItem{}
	for name, item := range dir.Dirs {
		cleanedItems = append(cleanedItems, ReducedItem{
			Name:    name,
			Size:    item.Size,
			ModTime: item.ModTime,
			Type:    item.Type,
		})
	}
	for name, item := range dir.Files {
		cleanedItems = append(cleanedItems, ReducedItem{
			Name:    name,
			Size:    item.Size,
			ModTime: item.ModTime,
			Type:    item.Type,
		})
	}
	dir.Items = cleanedItems
	return *dir, exists
}

// SetDirectoryInfo sets the directory information in the index.
func (si *Index) GetDirectoryInfo(adjustedPath string) (FileInfo, bool) {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	return *dir, exists
}

func (si *Index) RemoveDirectory(path string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	delete(si.Directories, path)
}

func (si *Index) UpdateCount(given string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	if given == "files" {
		si.NumFiles++
	} else if given == "dirs" {
		si.NumDirs++
	} else {
		log.Println("could not update unknown type: ", given)
	}
}

func (si *Index) resetCount() {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.NumDirs = 0
	si.NumFiles = 0
	si.inProgress = true
}

func GetIndex(root string) *Index {
	for _, index := range indexes {
		if index.Root == root {
			return index
		}
	}
	if settings.Config.Server.Root != "" {
		rootPath = settings.Config.Server.Root
	}
	newIndex := &Index{
		Root:        rootPath,
		Directories: map[string]*FileInfo{},
		NumDirs:     0,
		NumFiles:    0,
		inProgress:  false,
	}
	newIndex.Directories["/"] = &FileInfo{}
	indexesMutex.Lock()
	indexes = append(indexes, newIndex)
	indexesMutex.Unlock()
	return newIndex
}
