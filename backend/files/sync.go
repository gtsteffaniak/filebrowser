package files

import (
	"log"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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
	dirname := filepath.Base(dir.Path)
	if dirname == "." {
		dirname = "/"
	}

	if isDir {
		if len(dir.Items) > 0 {
			return dir, true
		}
		cleanedItems := []ReducedItem{}
		for _, item := range dir.Dirs {
			cleanedItems = append(cleanedItems, ReducedItem{
				Name:    item.Name,
				Size:    item.Size,
				ModTime: item.ModTime,
				Type:    "directory",
			})
		}

		cleanedItems = append(cleanedItems, dir.Files...)
		sort.Slice(cleanedItems, func(i, j int) bool {
			// Convert strings to integers for numeric sorting if both are numeric
			numI, errI := strconv.Atoi(cleanedItems[i].Name)
			numJ, errJ := strconv.Atoi(cleanedItems[j].Name)
			if errI == nil && errJ == nil {
				return numI < numJ
			}
			// Fallback to case-insensitive lexicographical sorting
			return strings.ToLower(cleanedItems[i].Name) < strings.ToLower(cleanedItems[j].Name)
		})
		dir.Type = "directory"
		dir.Items = cleanedItems
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
				Name:    item.Name,
				Size:    item.Size,
				ModTime: item.ModTime,
				Type:    item.Type,
				Path:    checkDir + "/" + item.Name,
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
