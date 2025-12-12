package indexing

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (idx *Index) UpdateMetadata(info *iteminfo.FileInfo) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.Directories[info.Path] = info
	return true
}

// DeleteMetadata removes the specified path from the index.
// If recursive is true and the path is a directory, it will also remove all subdirectories.
func (idx *Index) DeleteMetadata(path string, isDir bool, recursive bool) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Normalize the path - ensure trailing slash for directories
	indexPath := path
	if isDir {
		indexPath = utils.AddTrailingSlashIfNotExists(path)
	}

	// Clear cache entries
	joinedPath := filepath.Join(idx.Path, indexPath)
	RealPathCache.Delete(joinedPath)
	IsDirCache.Delete(joinedPath + ":isdir")

	if !isDir {
		// For files, remove from parent directory's Files slice
		parentPath := utils.AddTrailingSlashIfNotExists(filepath.Dir(strings.TrimSuffix(path, "/")))
		if parentDir, exists := idx.Directories[parentPath]; exists {
			fileName := filepath.Base(strings.TrimSuffix(path, "/"))
			for i, file := range parentDir.Files {
				if file.Name == fileName {
					// Remove file from slice
					parentDir.Files = append(parentDir.Files[:i], parentDir.Files[i+1:]...)
					break
				}
			}
		}
		return true
	}

	// For directories
	if recursive {
		// Remove all subdirectories that start with this path
		toDelete := []string{}
		for dirPath := range idx.Directories {
			// Match exact path or any subdirectory
			if dirPath == indexPath || strings.HasPrefix(dirPath, indexPath) {
				toDelete = append(toDelete, dirPath)
			}
		}
		for _, dirPath := range toDelete {
			delete(idx.Directories, dirPath)
			delete(idx.DirectoriesLedger, dirPath)
		}
	} else {
		// Just remove this specific directory
		delete(idx.Directories, indexPath)
		delete(idx.DirectoriesLedger, indexPath)
	}

	// Remove from parent directory's Folders slice
	if indexPath != "/" {
		parentPath := utils.AddTrailingSlashIfNotExists(filepath.Dir(strings.TrimSuffix(indexPath, "/")))
		if parentDir, exists := idx.Directories[parentPath]; exists {
			dirName := filepath.Base(strings.TrimSuffix(indexPath, "/"))
			for i, folder := range parentDir.Folders {
				if folder.Name == dirName {
					// Remove folder from slice
					parentDir.Folders = append(parentDir.Folders[:i], parentDir.Folders[i+1:]...)
					break
				}
			}
		}
	}

	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified file or directory in the index.
func (idx *Index) GetReducedMetadata(target string, isDir bool) (*iteminfo.FileInfo, bool) {
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
	if !exists {
		return nil, false
	}

	if isDir {
		return dir, true
	}
	// handle file
	baseName := filepath.Base(target)
	for _, item := range dir.Files {
		if item.Name == baseName {
			// Use path.Join to properly handle trailing slashes and avoid double slashes
			var fp string
			if checkDir == "/" {
				fp = "/" + item.Name
			} else {
				// Clean path to remove any trailing slashes before joining
				fp = strings.TrimSuffix(checkDir, "/") + "/" + item.Name
			}
			return &iteminfo.FileInfo{
				Path:     fp,
				ItemInfo: item.ItemInfo,
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

func GetIndex(name string) *Index {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()
	index, ok := indexes[name]
	if !ok {
		// try path if name fails
		// todo: update everywhere else so this isn't needed.
		source, ok := settings.Config.Server.SourceMap[name]
		if !ok {
			logger.Errorf("index %s not found", name)
			return nil
		}
		index, ok = indexes[source.Name]
		if !ok {
			logger.Errorf("index %s not found", name)
			return nil
		}
	}
	return index
}

// ReadOnlyOperation executes a function with read-only access to the index
// This provides a safe way to access index data without exposing internal structures
func (idx *Index) ReadOnlyOperation(fn func()) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	fn()
}

// GetDirectories returns the directories map for read-only access
// Should only be called within ReadOnlyOperation
func (idx *Index) GetDirectories() map[string]*iteminfo.FileInfo {
	return idx.Directories
}

func GetIndexInfo(sourceName string, forceCacheRefresh bool) (ReducedIndex, error) {
	idx, ok := indexes[sourceName]
	if !ok {
		return ReducedIndex{}, fmt.Errorf("index %s not found", sourceName)
	}

	// Only update disk total if cache is missing or explicitly forced
	// The "used" value comes from totalSize and is always current
	sourcePath := idx.Path
	cacheKey := "usageCache-" + sourceName
	if forceCacheRefresh {
		// Invalidate cache to force update
		utils.DiskUsageCache.Delete(cacheKey)
	}
	_, ok = utils.DiskUsageCache.Get(cacheKey)
	if !ok {
		// Only fetch disk total if not cached (this is expensive, so we cache it)
		totalBytes, err := getPartitionSize(sourcePath)
		if err != nil {
			idx.mu.Lock()
			idx.Status = UNAVAILABLE
			idx.mu.Unlock()
			return ReducedIndex{}, fmt.Errorf("error getting disk usage for %s: %v", sourcePath, err)
		}

		idx.SetUsage(totalBytes)
		utils.DiskUsageCache.Set(cacheKey, true)
	}

	// Build scanner info for client
	idx.mu.RLock()
	scannerInfos := make([]*ScannerInfo, 0, len(idx.scanners))
	for _, scanner := range idx.scanners {
		scannerInfos = append(scannerInfos, &ScannerInfo{
			Path:            scanner.scanPath,
			LastScanned:     scanner.lastScanned,
			Complexity:      scanner.complexity,
			CurrentSchedule: scanner.currentSchedule,
			QuickScanTime:   scanner.quickScanTime,
			FullScanTime:    scanner.fullScanTime,
			NumDirs:         scanner.numDirs,
			NumFiles:        scanner.numFiles,
		})
	}
	idx.mu.RUnlock()

	// Get fresh values from the index (with lock to ensure consistency)
	idx.mu.RLock()
	reducedIdx := idx.ReducedIndex
	// Ensure DiskUsed is up to date from totalSize
	reducedIdx.DiskUsed = idx.totalSize
	reducedIdx.DiskTotal = idx.DiskTotal
	reducedIdx.Scanners = scannerInfos
	idx.mu.RUnlock()
	return reducedIdx, nil
}
