package files

import (
	"log"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/settings"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (si *Index) UpdateMetadata(info *FileInfo) bool {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.Directories[info.Path] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetReducedMetadata(target string, isDir bool) (FileInfo, bool) {
	si.mu.Lock()
	defer si.mu.Unlock()
	checkDir := si.makeIndexPath(target)
	if !isDir {
		checkDir = si.makeIndexPath(filepath.Dir(target))
	}
	dir, exists := si.Directories[checkDir]
	if !exists {
		return FileInfo{}, false
	}
	dirname := filepath.Base(dir.Path)
	if dirname == "." {
		dirname = "/"
	}

	if isDir {
		cleanedItems := []ReducedItem{}
		cleanedItems = append(cleanedItems, dir.Dirs...)
		cleanedItems = append(cleanedItems, dir.Files...)
		dir.Type = "directory"
		dir.Items = cleanedItems
		return FileInfo{
			Name:    dirname,
			Size:    dir.Size,
			ModTime: dir.ModTime,
			Type:    "directory",
			Path:    checkDir,
			Items:   cleanedItems,
		}, true
	}
	// handle file
	if checkDir == "/" {
		checkDir = ""
	}
	baseName := filepath.Base(target)
	for _, item := range dir.Files {
		if item.Name == baseName {
			return FileInfo{
				Name:    item.Name,
				Size:    item.Size,
				ModTime: item.ModTime,
				Type:    item.Type,
				Path:    checkDir + "/" + item.Name,
			}, true
		}
	}
	return FileInfo{}, false

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
