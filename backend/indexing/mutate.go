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
// scanner parameter is optional - if nil (API refresh), directly inserts to database
func (idx *Index) UpdateMetadata(info *iteminfo.FileInfo, scanner *Scanner) bool {
	items := make([]*iteminfo.FileInfo, 0, len(info.Files)+len(info.Folders)+1)
	dirItem := *info
	items = append(items, &dirItem)

	// Check if we're in a batch scan (scanner provided)
	isBatchScan := scanner != nil

	if isBatchScan {
		// During recursive scans: only insert directory itself and direct files
		// Subdirectories will be inserted by their own recursive indexDirectory call
		// This avoids redundant inserts and maintains the in-memory size calculation pattern
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
	} else {
		// Not in batch scan (e.g., API-triggered updates): insert everything
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
	}

	// Use scanner-specific batch if available
	if scanner != nil {
		// Track that this directory was updated by the scan (for timestamp conflict detection)
		normalizedPath := utils.AddTrailingSlashIfNotExists(info.Path)
		idx.mu.Lock()
		idx.scanUpdatedPaths[normalizedPath] = true
		idx.mu.Unlock()

		// Accumulate items for bulk insert in scanner's batch
		scanner.batchItems = append(scanner.batchItems, items...)
		// Progressive flushing: flush every BATCH_SIZE items to keep memory bounded
		if len(scanner.batchItems) >= idx.db.BatchSize {
			scanner.flushBatch()
		}
		return true
	}

	// Not in a batch scan - insert immediately (e.g., API-triggered updates)
	if err := idx.db.BulkInsertItems(idx.Name, items); err != nil {
		logger.Errorf("Failed to update metadata for %s: %v", info.Path, err)
		return false
	}
	return true
}

// flushBatch writes all remaining batch items to the database (Scanner method)
// This is called at the end of a scan to flush any items that didn't reach the BATCH_SIZE threshold
func (s *Scanner) flushBatch() {
	items := s.batchItems
	s.batchItems = nil

	if len(items) == 0 {
		return
	}

	err := s.idx.db.BulkInsertItems(s.idx.Name, items)
	if err != nil {
		logger.Warningf("[DB_TX] Flush failed for scanner [%s] (%d items): %v", s.scanPath, len(items), err)
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

	// If this is a directory, populate size from in-memory map (prefer in-memory over DB)
	if item.Type == "directory" {
		dirPath := utils.AddTrailingSlashIfNotExists(checkPath)
		inMemSize, exists := idx.GetFolderSize(dirPath)
		if !exists {
			inMemSize = 0
		}
		if inMemSize > 0 || item.Size == 0 {
			item.Size = int64(inMemSize)
		}
	}

	return item, true
}

// raw directory info retrieval -- does not work on files, only returns a directory
func (idx *Index) GetMetadataInfo(target string, isDir bool) (*iteminfo.FileInfo, bool) {

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
	dir, err := idx.db.GetItem(idx.Name, checkDir)
	if err != nil {
		return nil, false
	}

	// If not found in DB during scan, item might not be indexed yet
	// (Batches are now scanner-specific, so we don't flush from API calls)
	if dir == nil {
		logger.Debugf("[GET_METADATA] Item %s not found in database", checkDir)
		return nil, false
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
			// Populate directory size from in-memory map (prefer in-memory over DB)
			childPath := utils.AddTrailingSlashIfNotExists(child.Path)
			inMemSize, exists := idx.GetFolderSize(childPath)
			if !exists {
				inMemSize = 0
			}
			if inMemSize > 0 || child.Size == 0 {
				// Use in-memory if available, or if DB also shows 0
				child.Size = int64(inMemSize)
			}
			// else: keep DB value if in-memory is 0 but DB has a value
			dir.Folders = append(dir.Folders, child.ItemInfo)
		} else {
			dir.Files = append(dir.Files, iteminfo.ExtendedItemInfo{ItemInfo: child.ItemInfo})
		}
	}

	// Populate the directory's own size from in-memory map (prefer in-memory over DB)
	inMemSize, exists := idx.GetFolderSize(checkDir)
	if !exists {
		inMemSize = 0
	}
	if inMemSize > 0 || dir.Size == 0 {
		dir.Size = int64(inMemSize)
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
func (idx *Index) ReadOnlyOperation(fn func()) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	fn()
}

// IterateFiles iterates over all files in the index and calls the callback function
func (idx *Index) IterateFiles(fn func(path, name string, size, modTime int64)) error {
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
			CurrentSchedule: scanner.currentSchedule,
			Stats: Stats{
				LastScanned:   scanner.lastScanned,
				Complexity:    scanner.complexity,
				QuickScanTime: scanner.quickScanTime,
				FullScanTime:  scanner.fullScanTime,
				NumDirs:       scanner.numDirs,
				NumFiles:      scanner.numFiles,
			},
		})
	}
	reducedIdx := idx.ReducedIndex
	// Compute DiskUsed from database (total size of all files)
	diskUsed, err := idx.db.GetTotalSize(idx.Name)
	if err != nil {
		logger.Errorf("Failed to get total size for index %s: %v", sourceName, err)
		diskUsed = 0
	}
	reducedIdx.DiskUsed = diskUsed
	reducedIdx.DiskTotal = idx.DiskTotal
	reducedIdx.Scanners = scannerInfos
	reducedIdx.NumDirs = idx.getNumDirsUnlocked()
	reducedIdx.NumFiles = idx.getNumFilesUnlocked()
	reducedIdx.QuickScanTime = idx.getQuickScanTimeUnlocked()
	reducedIdx.FullScanTime = idx.getFullScanTimeUnlocked()
	reducedIdx.Complexity = idx.getComplexityUnlocked()
	lastIndexed := idx.getLastIndexedUnlocked()
	reducedIdx.LastScanned = lastIndexed
	reducedIdx.LastIndexedUnix = lastIndexed.Unix()
	reducedIdx.Status = idx.getStatusUnlocked()
	idx.mu.RUnlock()
	return reducedIdx, nil
}
