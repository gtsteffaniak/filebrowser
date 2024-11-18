package files

import (
	"log"
	"path/filepath"
	"time"
	"sort"

	"github.com/gtsteffaniak/filebrowser/settings"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (si *Index) UpdateFileMetadata(target string, info FileInfo) bool {
	checkDir := si.makeIndexPath(target)
	si.mu.Lock()
	defer si.mu.Unlock()
	info.CacheTime = time.Now()
	si.Directories[checkDir] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetMetadataInfo(target string, isDir bool) (FileInfo, bool) {
	checkDir := si.makeIndexPath(target)
	if !isDir {
		checkDir = si.makeIndexPath(filepath.Dir(target))
	}
	si.mu.RLock()
	dir, exists := si.Directories[checkDir]
	si.mu.RUnlock()
	if !exists {
		return FileInfo{}, exists
	}
	if !isDir {
		baseName := filepath.Base(target)
		fileInfo, ok := dir.Files[baseName]
		return fileInfo, ok
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
		Directories: map[string]FileInfo{},
		NumDirs:     0,
		NumFiles:    0,
		inProgress:  false,
	}
	newIndex.Directories["/"] = FileInfo{}
	indexesMutex.Lock()
	indexes = append(indexes, newIndex)
	indexesMutex.Unlock()
	return newIndex
}
