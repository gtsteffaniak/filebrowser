package files

import (
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (si *Index) UpdateMetadata(target string, info *FileInfo) bool {
	if info.Type != "directory" {
		fmt.Println("not a directory")
		return false
	}
	checkDir := si.makeIndexPath(target)
	si.mu.Lock()
	defer si.mu.Unlock()
	info.CacheTime = time.Now()
	si.Directories[checkDir] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetMetadataInfo(target string, isDir bool) (*FileInfo, bool) {
	si.mu.RLock()
	defer si.mu.RUnlock()
	checkDir := si.makeIndexPath(target)
	if !isDir {
		checkDir = si.makeIndexPath(filepath.Dir(target))
	}
	dir, exists := si.Directories[checkDir]
	if !exists {
		return nil, false
	}
	if !isDir {
		baseName := filepath.Base(target)
		fileInfo, ok := dir.Files[baseName]
		if fileInfo.Path == "" {
			fileInfo.Path = dir.Path + "/" + fileInfo.Name
		}
		return fileInfo, ok
	}
	cleanedItems := []ReducedItem{}
	dir.Name = filepath.Base(dir.Path)
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
	sort.Slice(cleanedItems, func(i, j int) bool {
		return cleanedItems[i].Name < cleanedItems[j].Name
	})

	dir.Items = cleanedItems
	return dir, exists
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
