package files

import (
	"log"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (si *Index) UpdateFileMetadata(adjustedPath string, info FileInfo) bool {
	si.mu.Lock()
	defer si.mu.Unlock()
	dir, exists := si.Directories[adjustedPath]
	if !exists {
		si.Directories[adjustedPath] = FileInfo{}
	}
	return si.SetFileMetadata(adjustedPath, dir)
}

// SetFileMetadata sets the FileInfo for the specified directory in the index.
// internal use only
func (si *Index) SetFileMetadata(adjustedPath string, info FileInfo) bool {
	_, exists := si.Directories[adjustedPath]
	if !exists {
		return false
	}
	info.CacheTime = time.Now()
	si.Directories[adjustedPath] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetMetadataInfo(adjustedPath string) (FileInfo, bool) {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	if !exists {
		return dir, exists
	}
	// remove recursive items, we only want this directories direct files
	cleanedItems := []ReducedItem{}
	for _, item := range dir.Items {
		cleanedItems = append(cleanedItems, ReducedItem{
			Name:    item.Name,
			Size:    item.Size,
			IsDir:   item.IsDir,
			ModTime: item.ModTime,
			Type:    item.Type,
		})
	}
	dir.Items = nil
	dir.ReducedItems = cleanedItems
	realPath, _, _ := GetRealPath(adjustedPath)
	dir.Path = realPath
	return dir, exists
}

// SetDirectoryInfo sets the directory information in the index.
func (si *Index) SetDirectoryInfo(adjustedPath string, dir FileInfo) {
	si.mu.Lock()
	si.Directories[adjustedPath] = dir
	si.mu.Unlock()
}

// SetDirectoryInfo sets the directory information in the index.
func (si *Index) GetDirectoryInfo(adjustedPath string) (FileInfo, bool) {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
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
	indexesMutex.Lock()
	indexes = append(indexes, newIndex)
	indexesMutex.Unlock()
	return newIndex
}
