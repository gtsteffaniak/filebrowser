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

const BATCH_SIZE = 5000 // Progressive flush threshold for scanner batches

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
// During routine scans, items are accumulated in idx.batchItems and flushed at the end.
// For non-routine operations (API calls), items are inserted immediately.
func (idx *Index) UpdateMetadata(info *iteminfo.FileInfo) bool {
	// Quick nil check without mutex - db pointer is set once at init and never changes
	if idx.db == nil {
		return false
	}

	items := make([]*iteminfo.FileInfo, 0, len(info.Files)+1)

	// Convert relative path to absolute path for the directory
	absoluteDirPath := idx.MakeAbsolutePath(info.Path)
	dirItem := *info
	dirItem.Path = absoluteDirPath
	items = append(items, &dirItem)

	// Add files to the bulk insert with absolute paths
	for i := range info.Files {
		f := &info.Files[i]
		// Construct relative path for the file
		relativeFilePath := strings.TrimRight(info.Path, "/") + "/" + f.Name
		// Convert to absolute path
		absoluteFilePath := idx.MakeAbsolutePath(relativeFilePath)

		fileItem := &iteminfo.FileInfo{
			ItemInfo: f.ItemInfo,
			Path:     absoluteFilePath,
		}
		items = append(items, fileItem)
	}

	// Check if we're in a batch scan (batchItems is initialized)
	idx.mu.Lock()
	if idx.batchItems != nil {
		// Accumulate items for bulk insert
		idx.batchItems = append(idx.batchItems, items...)

		// Progressive flushing: flush every 5000 items to keep memory bounded
		// and make items available in the index sooner
		if len(idx.batchItems) >= 5000 {
			itemsToFlush := idx.batchItems
			idx.batchItems = make([]*iteminfo.FileInfo, 0, 5000) // Reset for next batch
			idx.mu.Unlock()

			// Flush in background to avoid blocking scanner
			// Even if flush fails due to DB contention, scanner continues
			go func(items []*iteminfo.FileInfo) {
				logger.Debugf("[DB_TX] Progressive flush: starting async flush of %d items", len(items))
				err := idx.db.BulkInsertItems(items)
				if err != nil {
					logger.Warningf("[DB_TX] Progressive flush failed (%d items): %v - continuing scan", len(items), err)
				} else {
					logger.Debugf("[DB_TX] Progressive flush: SUCCESS - %d items", len(items))
				}
			}(itemsToFlush)
			return true
		}
		idx.mu.Unlock()
		return true
	}
	idx.mu.Unlock()

	// Not in a batch scan - insert immediately (e.g., API-triggered updates)
	if err := idx.db.BulkInsertItems(items); err != nil {
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
	idx.batchItems = nil      // Clear the batch
	idx.isRoutineScan = false // Reset routine flag
	idx.mu.Unlock()

	if len(items) == 0 {
		return
	}

	logger.Debugf("[DB_TX] Final flush: %d remaining items from scan", len(items))

	// Flush synchronously at end of scan, but with fast-fail on contention
	// Scanner has already completed, so we just log if this fails
	err := idx.db.BulkInsertItems(items)
	if err != nil {
		logger.Warningf("[DB_TX] Final flush failed (%d items): %v", len(items), err)
	} else {
		logger.Debugf("[DB_TX] Final flush: SUCCESS - %d items", len(items))
	}
}

// DeleteMetadata removes the specified path from the index.
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

	// Convert to absolute path
	absolutePath := idx.MakeAbsolutePath(indexPath)

	if err := idx.db.DeleteItem(absolutePath, recursive); err != nil {
		logger.Errorf("Failed to delete metadata for %s: %v", absolutePath, err)
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

	checkPath := idx.MakeIndexPath(target)
	if !isDir {
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

	// Convert to absolute path
	absolutePath := idx.MakeAbsolutePath(checkPath)

	item, err := idx.db.GetItem(absolutePath)
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

	checkDir := idx.MakeIndexPath(target)
	if !isDir {
		checkDir = idx.MakeIndexPath(filepath.Dir(target))
	}
	checkDir = utils.AddTrailingSlashIfNotExists(checkDir)

	if checkDir == "" {
		checkDir = "/"
	}

	// Convert to absolute path
	absoluteDirPath := idx.MakeAbsolutePath(checkDir)

	// Get directory item
	dir, err := idx.db.GetItem(absoluteDirPath)
	if err != nil {
		return nil, false
	}

	// Guard against nil item (can happen if DB was busy/locked and GetItem returned nil, nil)
	if dir == nil {
		return nil, false
	}

	// Get children
	children, err := idx.db.GetDirectoryChildren(absoluteDirPath)
	if err != nil {
		logger.Errorf("Failed to get children for %s: %v", absoluteDirPath, err)
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
