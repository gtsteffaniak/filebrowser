package files

import (
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/settings"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (idx *Index) UpdateMetadata(info *FileInfo) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.Directories[info.Path] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (idx *Index) GetReducedMetadata(target string, isDir bool) (*FileInfo, bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	checkDir := idx.makeIndexPath(target)
	if !isDir {
		checkDir = idx.makeIndexPath(filepath.Dir(target))
	}
	dir, exists := idx.Directories[checkDir]
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
func (idx *Index) GetMetadataInfo(target string, isDir bool) (*FileInfo, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	checkDir := idx.makeIndexPath(target)
	if !isDir {
		checkDir = idx.makeIndexPath(filepath.Dir(target))
	}
	dir, exists := idx.Directories[checkDir]
	return dir, exists
}

func (idx *Index) RemoveDirectory(path string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.NumDeleted++
	delete(idx.Directories, path)
}

func GetIndex(name string) *Index {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()
	index, ok := indexes[name]
	if !ok {
		return &Index{
			Source: settings.Source{Name: "default", Path: "."},
		}
	}
	return index
}
