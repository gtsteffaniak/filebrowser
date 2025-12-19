package indexing

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (idx *Index) UpdateMetadata(info *iteminfo.FileInfo) bool {
	// Quick nil check without mutex - db pointer is set once at init and never changes
	if idx.db == nil {
		return false
	}

	items := make([]*iteminfo.FileInfo, 0, len(info.Files)+len(info.Folders)+1)
	dirItem := *info
	items = append(items, &dirItem)

	// Add folders to the bulk insert with index paths
	for i := range info.Folders {
		folder := &info.Folders[i]
		// Construct index path for the folder (with trailing slash)
		folderPath := strings.TrimRight(info.Path, "/") + "/" + folder.Name + "/"

		folderItem := &iteminfo.FileInfo{
			ItemInfo: *folder,
			Path:     folderPath, // Store index path with trailing slash
		}
		items = append(items, folderItem)
	}

	// Add files to the bulk insert with index paths
	for i := range info.Files {
		f := &info.Files[i]
		// Construct index path for the file
		filePath := strings.TrimRight(info.Path, "/") + "/" + f.Name

		fileItem := &iteminfo.FileInfo{
			ItemInfo: f.ItemInfo,
			Path:     filePath, // Store index path, not absolute path
		}
		items = append(items, fileItem)
	}

	// Check if we're in a batch scan (batchItems is initialized)
	idx.mu.Lock()
	if idx.batchItems != nil {
		// Accumulate items for bulk insert
		idx.batchItems = append(idx.batchItems, items...)

		// Progressive flushing: flush every BATCH_SIZE items to keep memory bounded
		// Synchronous flush ensures only ONE batch in memory at a time
		if len(idx.batchItems) >= idx.db.BatchSize {
			itemsToFlush := idx.batchItems
			idx.batchItems = make([]*iteminfo.FileInfo, 0, idx.db.BatchSize)
			sourceName := idx.Name
			numItems := len(itemsToFlush)
			idx.mu.Unlock()

			// Synchronous flush - blocks scanner until DB write completes
			// This ensures only BatchSize items max in memory at any time
			err := idx.db.BulkInsertItems(sourceName, itemsToFlush)
			if err != nil {
				logger.Warningf("[DB_TX] Progressive flush failed (%d items): %v - continuing scan", numItems, err)
			}
			return true
		}
		idx.mu.Unlock()
		return true
	}
	idx.mu.Unlock()

	// Not in a batch scan - insert immediately (e.g., API-triggered updates)
	if err := idx.db.BulkInsertItems(idx.Name, items); err != nil {
		logger.Errorf("Failed to update metadata for %s: %v", info.Path, err)
		return false
	}
	return true
}

// flushBatch writes all remaining batch items to the database
// This is called at the end of a scan to flush any items that didn't reach the BATCH_SIZE threshold
func (idx *Index) flushBatch() {
	idx.mu.Lock()
	items := idx.batchItems
	idx.batchItems = nil
	idx.mu.Unlock()

	if len(items) == 0 {
		return
	}

	err := idx.db.BulkInsertItems(idx.Name, items)
	if err != nil {
		logger.Warningf("[DB_TX] Final flush failed (%d items): %v", len(items), err)
	}
}

// DeleteMetadata removes the specified path from the index.
// SQLite handles locking automatically, so no mutex needed.
func (idx *Index) DeleteMetadata(path string, isDir bool, recursive bool) bool {
	// Normalize the path - ensure trailing slash for directories
	indexPath := path
	if isDir {
		indexPath = utils.AddTrailingSlashIfNotExists(path)
	} else {
		indexPath = strings.TrimSuffix(indexPath, "/")
	}
	// indexPath is already an index path (relative to source root)
	if err := idx.db.DeleteItem(idx.Name, indexPath, recursive); err != nil {
		logger.Errorf("Failed to delete metadata for %s: %v", indexPath, err)
		return false
	}

	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified file or directory in the index.
func (idx *Index) GetReducedMetadata(target string, isDir bool) (*iteminfo.FileInfo, bool) {
	// Quick nil check without mutex - db pointer is set once at init and never changes
	if idx.db == nil {
		return nil, false
	}

	checkPath := idx.MakeIndexPath(target, isDir)

	if checkPath == "" {
		checkPath = "/"
	}

	// checkPath is already an index path (relative to source root)
	item, err := idx.db.GetItem(idx.Name, checkPath)
	if err != nil {
		return nil, false
	}

	// Guard against nil item (can happen if DB was busy/locked and GetItem returned nil, nil)
	if item == nil {
		return nil, false
	}

	return item, true
}

// raw directory info retrieval -- does not work on files, only returns a directory
func (idx *Index) GetMetadataInfo(target string, isDir bool) (*iteminfo.FileInfo, bool) {
	// Quick nil check without mutex - db pointer is set once at init and never changes
	if idx.db == nil {
		return nil, false
	}

	var checkDir string
	if !isDir {
		checkDir = idx.MakeIndexPath(filepath.Dir(target), true)
	} else {
		checkDir = idx.MakeIndexPath(target, true)
	}

	if checkDir == "" {
		checkDir = "/"
	}

	// checkDir is already an index path (relative to source root)
	// Try to get from DB first
	dir, err := idx.db.GetItem(idx.Name, checkDir)
	if err != nil {
		return nil, false
	}

	// If not found in DB, check if we have pending batch items and flush everything
	// Flushing everything releases memory sooner and is simpler than selective flushing
	if dir == nil {
		idx.mu.Lock()
		hasBatchItems := len(idx.batchItems) > 0
		idx.mu.Unlock()

		if hasBatchItems {
			// Flush entire batch to release memory and make data available
			idx.flushBatch()
			// Try again after flush
			dir, err = idx.db.GetItem(idx.Name, checkDir)
			if err != nil || dir == nil {
				return nil, false
			}
		} else {
			// Not in batch either, doesn't exist
			return nil, false
		}
	}

	// Get children
	children, err := idx.db.GetDirectoryChildren(idx.Name, checkDir)
	if err != nil {
		logger.Errorf("Failed to get children for %s: %v", checkDir, err)
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
	rows, err := idx.db.Query("SELECT path, name, size, mod_time FROM index_items WHERE source = ? AND is_dir = 0", idx.Name)
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
		totalBytes, err := fileutils.GetPartitionSize(sourcePath)
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
