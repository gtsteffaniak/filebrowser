package indexing

import (
	"fmt"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/cache"
	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/shirou/gopsutil/v3/disk"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (idx *Index) UpdateMetadata(info *iteminfo.FileInfo) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.Directories[info.Path] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified file or directory in the index.
func (idx *Index) GetReducedMetadata(target string, isDir bool) (*iteminfo.FileInfo, bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	checkDir := idx.MakeIndexPath(target)
	if !isDir {
		checkDir = idx.MakeIndexPath(filepath.Dir(target))
	}
	if checkDir == "" {
		checkDir = "/"
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
			return &iteminfo.FileInfo{
				Path:     checkDir + "/" + item.Name,
				ItemInfo: item,
			}, true
		}
	}
	return nil, false

}

// raw directory info retrieval -- does not work on files, only returns a directory
func (idx *Index) GetMetadataInfo(target string, isDir bool) (*iteminfo.FileInfo, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	checkDir := idx.MakeIndexPath(target)
	if !isDir {
		checkDir = idx.MakeIndexPath(filepath.Dir(target))
	}
	if checkDir == "" {
		checkDir = "/"
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
		logger.Error(fmt.Sprintf("index %s not found", name))
	}
	return index
}

func GetIndexesInfo(sources ...string) map[string]ReducedIndex {
	// update usage if needed
	for _, source := range sources {
		s, ok := settings.Config.Server.NameToSource[source]
		if !ok {
			logger.Error(fmt.Sprintf("source %s not found", source))
			continue
		}
		sourcePath := s.Path
		cacheKey := "usageCache-" + source
		_, ok = cache.DiskUsage.Get(cacheKey).(bool)
		if !ok {
			usage, err := disk.Usage(sourcePath)
			if err != nil {
				logger.Error(fmt.Sprintf("error getting disk usage for %s: %v", sourcePath, err))
			}
			latestUsage := DiskUsage{
				Total: usage.Total,
				Used:  usage.Used,
			}
			SetUsage(source, latestUsage)
			cache.DiskUsage.Set(cacheKey, true)
		}
	}
	indexesMutex.RLock()
	defer indexesMutex.RUnlock()
	reducedIndexes := make(map[string]ReducedIndex)
	for k, v := range indexes {
		reducedIndexes[k] = v.ReducedIndex
	}
	return reducedIndexes
}
