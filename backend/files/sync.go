package files

import (
	"io/fs"
	"log"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
)

// GetFileMetadata retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetFileMetadata(adjustedPath string) (FileInfo, bool) {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	if exists {
		// Initialize the Metadata map if it is nil
		if dir.Metadata == nil {
			dir.Metadata = make(map[string]FileInfo)
			si.SetDirectoryInfo(adjustedPath, dir)
			return FileInfo{}, false
		} else {
			return dir.Metadata[adjustedPath], true
		}
	}
	return FileInfo{}, false
}

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (si *Index) UpdateFileMetadata(adjustedPath string, info FileInfo) bool {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	if !exists {
		// Initialize the Metadata map if it is nil
		if dir.Metadata == nil {
			dir.Metadata = make(map[string]FileInfo)
		}
		si.Directories[adjustedPath] = dir
		// Release the read lock before calling SetFileMetadata
	}
	return si.SetFileMetadata(adjustedPath, info)
}

// SetFileMetadata sets the FileInfo for the specified directory in the index.
func (si *Index) SetFileMetadata(adjustedPath string, info FileInfo) bool {
	si.mu.Lock()
	defer si.mu.Unlock()
	_, exists := si.Directories[adjustedPath]
	if !exists {
		return false
	}
	info.CacheTime = time.Now()
	si.Directories[adjustedPath].Metadata[adjustedPath] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified directory in the index.
func (si *Index) GetMetadataInfo(adjustedPath string) (FileInfo, bool) {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	if exists {
		// Initialize the Metadata map if it is nil
		if dir.Metadata == nil {
			dir.Metadata = make(map[string]FileInfo)
			si.SetDirectoryInfo(adjustedPath, dir)
		}
		info, metadataExists := dir.Metadata[adjustedPath]
		return info, metadataExists
	}
	return FileInfo{}, false
}

// SetDirectoryInfo sets the directory information in the index.
func (si *Index) SetDirectoryInfo(adjustedPath string, dir Directory) {
	si.mu.Lock()
	si.Directories[adjustedPath] = dir
	si.mu.Unlock()
}

// SetDirectoryInfo sets the directory information in the index.
func (si *Index) GetDirectoryInfo(adjustedPath string) (Directory, bool) {
	si.mu.RLock()
	dir, exists := si.Directories[adjustedPath]
	si.mu.RUnlock()
	if exists {
		return dir, true
	}
	return Directory{}, false
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
		Directories: make(map[string]Directory), // Initialize the map
		NumDirs:     0,
		NumFiles:    0,
		inProgress:  false,
	}
	indexesMutex.Lock()
	indexes = append(indexes, newIndex)
	indexesMutex.Unlock()
	return newIndex
}

func (si *Index) UpdateQuickList(files []fs.FileInfo) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.quickList = []File{}
	for _, file := range files {
		newFile := File{
			Name:  file.Name(),
			IsDir: file.IsDir(),
		}
		si.quickList = append(si.quickList, newFile)
	}
}

func (si *Index) UpdateQuickListForTests(files []File) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.quickList = []File{}
	for _, file := range files {
		newFile := File{
			Name:  file.Name,
			IsDir: file.IsDir,
		}
		si.quickList = append(si.quickList, newFile)
	}
}

func (si *Index) GetQuickList() []File {
	si.mu.Lock()
	defer si.mu.Unlock()
	newQuickList := si.quickList
	return newQuickList
}
