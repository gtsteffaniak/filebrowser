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
	
	if idx.db == nil {
		return false
	}

	items := make([]*iteminfo.FileInfo, 0, len(info.Files)+1)
	items = append(items, info)

	// Add files to the bulk insert
	for i := range info.Files {
		f := &info.Files[i]
		// Construct full path for the file
		filePath := strings.TrimRight(info.Path, "/") + "/" + f.Name
		
		fileItem := &iteminfo.FileInfo{
			ItemInfo: f.ItemInfo,
			Path:     filePath,
		}
		items = append(items, fileItem)
	}

	if err := idx.db.BulkInsertItems(items); err != nil {
		logger.Errorf("Failed to update metadata for %s: %v", info.Path, err)
		return false
	}
	
	return true
}

// DeleteMetadata removes the specified path from the index.
// If recursive is true and the path is a directory, it will also remove all subdirectories.
// NOTE: path should already be an index path (with trailing slash for directories)
func (idx *Index) DeleteMetadata(path string, isDir bool, recursive bool) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	
	if idx.db == nil {
		return false
	}

	// Normalize the path - ensure trailing slash for directories
	indexPath := path
	if isDir {
		indexPath = utils.AddTrailingSlashIfNotExists(path)
	}

	if err := idx.db.DeleteItem(indexPath, recursive); err != nil {
		logger.Errorf("Failed to delete metadata for %s: %v", indexPath, err)
		return false
	}
	
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified file or directory in the index.
func (idx *Index) GetReducedMetadata(target string, isDir bool) (*iteminfo.FileInfo, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	if idx.db == nil {
		return nil, false
	}

	checkPath := idx.MakeIndexPath(target)
	if !isDir {
		// For files, we want the file itself, not the parent directory
		// But MakeIndexPath adds trailing slash for directories.
		// If target is file, MakeIndexPath might treat it as directory if we don't be careful.
		// Actually MakeIndexPath implementation:
		// path = utils.AddTrailingSlashIfNotExists(path)
		// So it always adds trailing slash!
		// We need to strip it if it's a file.
		if strings.HasSuffix(checkPath, "/") {
			checkPath = strings.TrimSuffix(checkPath, "/")
		}
	} else {
		// Ensure trailing slash for directory
		checkPath = utils.AddTrailingSlashIfNotExists(checkPath)
	}
	
	if checkPath == "" {
		checkPath = "/"
	}

	item, err := idx.db.GetItem(checkPath)
	if err != nil {
		return nil, false
	}
	return item, true
}

// raw directory info retrieval -- does not work on files, only returns a directory
func (idx *Index) GetMetadataInfo(target string, isDir bool) (*iteminfo.FileInfo, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	if idx.db == nil {
		return nil, false
	}

	checkDir := idx.MakeIndexPath(target)
	if !isDir {
		checkDir = idx.MakeIndexPath(filepath.Dir(target))
	}
	checkDir = utils.AddTrailingSlashIfNotExists(checkDir)
	
	if checkDir == "" {
		checkDir = "/"
	}
	
	// Get directory item
	dir, err := idx.db.GetItem(checkDir)
	if err != nil {
		return nil, false
	}
	
	// Get children
	children, err := idx.db.GetDirectoryChildren(checkDir)
	if err != nil {
		logger.Errorf("Failed to get children for %s: %v", checkDir, err)
		// Return partial info? Or fail?
		// Existing behavior returns nil if not exists.
		// If dir exists but children fail, maybe return empty dir?
		return dir, true
	}
	
	// Populate Files and Folders
	for _, child := range children {
		if child.Type == "directory" {
			dir.Folders = append(dir.Folders, child.ItemInfo)
		} else {
			dir.Files = append(dir.Files, iteminfo.ExtendedItemInfo{ItemInfo: child.ItemInfo})
		}
	}
	
	return dir, true
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

// IterateFiles iterates over all files in the index and calls the callback function
func (idx *Index) IterateFiles(fn func(path, name string, size, modTime int64)) error {
	if idx.db == nil {
		return nil
	}
	
	rows, err := idx.db.Query("SELECT path, name, size, mod_time FROM index_items WHERE is_dir = 0")
	if err != nil {
		return err
	}
	defer rows.Close()
	
	for rows.Next() {
		var path, name string
		var size, modTime int64
		if err := rows.Scan(&path, &name, &size, &modTime); err != nil {
			return err
		}
		fn(path, name, size, modTime)
	}
	return nil
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
