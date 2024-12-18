package files

import (
	"path/filepath"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (si *Index) UpdateMetadata(info *FileInfo) bool {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.Directories[info.Path] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetReducedMetadata(target string, isDir bool) (*FileInfo, bool) {
	si.mu.Lock()
	defer si.mu.Unlock()
	checkDir := si.makeIndexPath(target)
	if !isDir {
		checkDir = si.makeIndexPath(filepath.Dir(target))
	}
	dir, exists := si.Directories[checkDir]
	if !exists {
		return nil, false
	}

	if isDir {
		return dir, true
	}
	// handle file
	if checkDir == "/" {
		checkDir = ""
	}
	baseName := filepath.Base(target)
	for _, item := range dir.Files {
		if item.Name == baseName {
			return &FileInfo{
				Path:     checkDir + "/" + item.Name,
				ItemInfo: item,
			}, true
		}
	}
	return nil, false

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
	return dir, exists
}

func (si *Index) RemoveDirectory(path string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.NumDeleted++
	delete(si.Directories, path)
}

func GetIndex(name string) *Index {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()
	index, ok := indexes[name]
	if !ok {
		return nil
	}
	return index
}

func getRoot(name string) string {
	index := GetIndex(name)
	if index == nil {
		return ""
	}
	return index.Root
}
