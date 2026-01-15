package indexing

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

type Scanner struct {
	scanPath string

	currentSchedule int
	smartModifier   time.Duration
	complexity      uint
	fullScanCounter int
	nextSleepTime   time.Duration
	filesChanged    bool
	lastScanned     time.Time
	quickScanTime   int
	fullScanTime    int
	scanStartTime   int64
	isScanning      bool // True when scanner is actively scanning or waiting for mutex

	numDirs  uint64
	numFiles uint64

	// Per-scanner state (not shared with other scanners)
	processedInodes map[uint64]struct{}  // Track inodes to detect hardlinks
	foundHardLinks  map[string]uint64    // Path -> size for hardlinks found
	batchItems      []*iteminfo.FileInfo // Accumulates items for bulk insert

	idx *Index

	stopChan chan struct{}
	stopOnce sync.Once // Ensures stopChan is only closed once
}

// start begins the scanner's main loop
func (s *Scanner) start() {
	// Wait a bit to stagger child scanner initial scans (root goes first)
	if s.scanPath != "/" {
		time.Sleep(500 * time.Millisecond)
	}
	s.tryAcquireAndScan()

	for {
		// Check if directory still exists (for non-root scanners)
		if s.scanPath != "/" && !s.directoryExists() {
			logger.Debugf("Scanner [%s] stopping: directory no longer exists", s.scanPath)
			s.removeSelf()
			return
		}

		// Use the sleep time calculated by updateSchedule() (based on OLD schedule before increment)
		sleepTime := s.nextSleepTime
		if sleepTime == 0 {
			// Fallback for first run or if not set
			sleepTime = s.calculateSleepTime()
		}

		select {
		case <-s.stopChan:
			return

		case <-time.After(sleepTime):
			s.tryAcquireAndScan()
		}
	}
}

// tryAcquireAndScan attempts to acquire the global scan mutex and run a scan
func (s *Scanner) tryAcquireAndScan() {
	// Ensure isScanning is cleared even if we panic
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("[%s] Scanner panic recovered: %v", s.scanPath, r)
		}
		s.idx.mu.Lock()
		s.isScanning = false
		s.idx.mu.Unlock()
		err := s.idx.PostScan()
		if err != nil {
			logger.Errorf("Failed to post scan: %v", err)
		}
	}()

	// Set isScanning flag for all scanners
	s.idx.mu.Lock()
	s.isScanning = true
	s.idx.mu.Unlock()

	// Root scanner can run independently since it's non-recursive
	// Child scanners must acquire mutex to prevent concurrent child scans
	if s.scanPath != "/" {
		s.idx.childScanMutex.Lock()
		defer s.idx.childScanMutex.Unlock()
	}

	quick := s.fullScanCounter > 0 && s.fullScanCounter < 5
	s.fullScanCounter++
	if s.fullScanCounter >= 5 {
		s.fullScanCounter = 0
	}

	s.runIndexing(quick)
	s.updateSchedule()
}

// runIndexing performs the actual indexing work
func (s *Scanner) runIndexing(quick bool) {
	if s.scanPath == "/" {
		s.runRootScan(quick)
	} else {
		s.runChildScan(quick)
	}
}

func (s *Scanner) runRootScan(quick bool) {
	startTime := time.Now()

	if quick {
		s.runQuickScanRoot()
		s.quickScanTime = int(time.Since(startTime).Seconds())
	} else {
		s.runFullScanRoot()
		s.fullScanTime = int(time.Since(startTime).Seconds())
		s.updateComplexity()
	}

	s.lastScanned = time.Now()
	s.checkForNewChildDirectories()
	if err := s.idx.db.ShrinkMemory(); err != nil {
		logger.Errorf("Failed to shrink memory: %v", err)
	}
}

func (s *Scanner) runFullScanRoot() {
	config := Options{
		Recursive:     false,
		IsRoutineScan: true,
	}

	// Reset counters and initialize state for full scan
	s.numDirs = 0
	s.numFiles = 0
	s.scanStartTime = time.Now().Unix()
	s.idx.mu.Lock()
	if s.idx.scanSessionStartTime == 0 {
		s.idx.scanSessionStartTime = s.scanStartTime
	}
	s.idx.mu.Unlock()

	s.processedInodes = make(map[uint64]struct{})
	s.foundHardLinks = make(map[string]uint64)

	batchSize := s.idx.db.BatchSize
	s.batchItems = make([]*iteminfo.FileInfo, 0, batchSize)
	s.filesChanged = false

	_, _, err := s.idx.indexDirectory("/", config, s)
	if err != nil {
		logger.Errorf("Root scanner error: %v", err)
	}

	s.flushBatch()
	s.purgeStaleEntries(true, false) // isRoot=true, isQuickScan=false
	s.syncStatsWithDB()

	if s.processedInodes != nil {
		s.processedInodes = nil
		s.foundHardLinks = nil
	}

	s.idx.mu.Lock()
	s.idx.scanUpdatedPaths = make(map[string]bool)
	s.idx.mu.Unlock()
}

func (s *Scanner) runQuickScanRoot() {
	s.scanStartTime = time.Now().Unix()
	s.filesChanged = false

	// Get root-level folders from folderSizes map
	s.idx.folderSizesMu.RLock()
	var foldersToCheck []string
	for path := range s.idx.folderSizes {
		// Root scanner: only check direct children (one slash after root, like "/subdir/")
		if strings.Count(path, "/") == 2 && path != "/" {
			foldersToCheck = append(foldersToCheck, path)
		}
	}
	s.idx.folderSizesMu.RUnlock()

	batchSize := s.idx.db.BatchSize
	s.batchItems = make([]*iteminfo.FileInfo, 0, batchSize)

	// Track stats
	unchangedCount := 0
	changedCount := 0
	errorCount := 0

	// Check each folder's modtime
	for _, folderPath := range foldersToCheck {
		changed, err := s.checkFolderModtime(folderPath)
		if err != nil {
			errorCount++
		} else if changed {
			changedCount++
		} else {
			unchangedCount++
		}
	}

	s.flushBatch()
	s.purgeStaleEntries(true, true) // isRoot=true, isQuickScan=true
	s.syncStatsWithDB()
}

func (s *Scanner) runChildScan(quick bool) {
	startTime := time.Now()

	if quick {
		// Quick scan: iterate folderSizes, check modtimes only (257x faster!)
		s.runQuickScanChild()
		s.quickScanTime = int(time.Since(startTime).Seconds())
	} else {
		// Full scan: walk filesystem recursively, read all directory contents
		s.runFullScanChild()
		s.fullScanTime = int(time.Since(startTime).Seconds())
		s.updateComplexity()
	}

	s.lastScanned = time.Now()
}

func (s *Scanner) runFullScanChild() {
	config := Options{
		Recursive:     true,
		IsRoutineScan: true,
	}

	// Reset counters and initialize state for full scan
	s.numDirs = 0
	s.numFiles = 0
	s.scanStartTime = time.Now().Unix()
	s.idx.mu.Lock()
	if s.idx.scanSessionStartTime == 0 {
		s.idx.scanSessionStartTime = s.scanStartTime
	}
	s.idx.mu.Unlock()

	s.processedInodes = make(map[uint64]struct{})
	s.foundHardLinks = make(map[string]uint64)
	s.filesChanged = false

	batchSize := s.idx.db.BatchSize
	s.batchItems = make([]*iteminfo.FileInfo, 0, batchSize)

	_, _, err := s.idx.indexDirectory(s.scanPath, config, s)
	if err != nil {
		logger.Errorf("Scanner [%s] error: %v", s.scanPath, err)
	} else if config.Recursive && config.IsRoutineScan {
		s.idx.mu.Lock()
		s.idx.incrementScannerDirsUnlocked()
		s.idx.mu.Unlock()
	}

	s.flushBatch()
	s.purgeStaleEntries(false, false) // isRoot=false, isQuickScan=false
	s.syncStatsWithDB()

	if s.scanPath != "/" {
		if err := s.idx.SyncFolderSizesToDB(); err != nil {
			logger.Errorf("[%s] Failed to sync folder sizes: %v", s.scanPath, err)
		}
	}

	if s.processedInodes != nil {
		s.processedInodes = nil
		s.foundHardLinks = nil
	}

	s.idx.mu.Lock()
	for path := range s.idx.scanUpdatedPaths {
		if strings.HasPrefix(path, s.scanPath) {
			delete(s.idx.scanUpdatedPaths, path)
		}
	}
	s.idx.mu.Unlock()
}

func (s *Scanner) runQuickScanChild() {
	s.scanStartTime = time.Now().Unix()
	s.filesChanged = false

	// Get folders in this scanner's scope from folderSizes map
	s.idx.folderSizesMu.RLock()
	var foldersToCheck []string
	for path := range s.idx.folderSizes {
		// Child scanner: check all paths under its scope
		if strings.HasPrefix(path, s.scanPath) && path != s.scanPath {
			foldersToCheck = append(foldersToCheck, path)
		}
	}
	s.idx.folderSizesMu.RUnlock()

	batchSize := s.idx.db.BatchSize
	s.batchItems = make([]*iteminfo.FileInfo, 0, batchSize)

	// Update the scanner's own root folder to prevent it from being purged
	// (child folders are updated via checkFolderModtime, but the scanner's root needs explicit update)
	fullScanPath := filepath.Join(s.idx.Path, s.scanPath)
	if info, err := os.Stat(fullScanPath); err == nil {
		rootFolderInfo := &iteminfo.FileInfo{
			Path: s.scanPath,
			ItemInfo: iteminfo.ItemInfo{
				Name:       filepath.Base(strings.TrimSuffix(s.scanPath, "/")),
				Size:       0, // Folder size managed in-memory via folderSizes map
				ModTime:    info.ModTime(),
				Type:       "directory",
				Hidden:     false,
				HasPreview: false,
			},
		}
		// Add to batch for bulk update
		s.batchItems = append(s.batchItems, rootFolderInfo)
	}

	// Track stats
	unchangedCount := 0
	changedCount := 0
	errorCount := 0

	// Check each folder's modtime
	for _, folderPath := range foldersToCheck {
		changed, err := s.checkFolderModtime(folderPath)
		if err != nil {
			errorCount++
		} else if changed {
			changedCount++
		} else {
			unchangedCount++
		}
	}

	s.flushBatch()
	s.purgeStaleEntries(false, true) // isRoot=false, isQuickScan=true
	s.syncStatsWithDB()

}

// checkForNewChildDirectories detects new top-level directories and creates scanners for them
// Also detects deleted directories and signals their scanners to stop
func (s *Scanner) checkForNewChildDirectories() {
	if s.scanPath != "/" {
		return
	}

	// Get current top-level directories from filesystem (already filtered by exclusion rules)
	currentDirs := s.getTopLevelDirs()
	currentDirsMap := make(map[string]bool)
	for _, dir := range currentDirs {
		currentDirsMap[dir] = true
	}

	// Check which scanners already exist
	s.idx.mu.RLock()
	existingScanners := make(map[string]*Scanner)
	for path, scanner := range s.idx.scanners {
		if path != "/" { // Don't check root scanner
			existingScanners[path] = scanner
		}
	}
	s.idx.mu.RUnlock()

	for path, scanner := range existingScanners {
		if !currentDirsMap[path] {
			logger.Debugf("Directory [%s] no longer exists, stopping scanner", path)
			scanner.stop()
			// Remove from map immediately to prevent stale scanner references
			s.idx.mu.Lock()
			delete(s.idx.scanners, path)
			s.idx.mu.Unlock()
		}
	}

	// Create scanner for any new directories (getTopLevelDirs already filtered excluded dirs)
	for _, dirPath := range currentDirs {
		_, exists := existingScanners[dirPath]
		if !exists && dirPath != "/" {
			logger.Debugf("Detected new directory, creating scanner: [%s]", dirPath)
			newScanner := s.idx.createChildScanner(dirPath)

			s.idx.mu.Lock()
			s.idx.scanners[dirPath] = newScanner
			s.idx.mu.Unlock()

			go newScanner.start()
		}
	}
}

// getTopLevelDirs returns a list of top-level directory paths in the root
func (s *Scanner) getTopLevelDirs() []string {
	dirs := []string{}
	dir, err := os.Open(s.idx.Path)
	if err != nil {
		logger.Errorf("Failed to open root directory: %v", err)
		return dirs
	}
	defer dir.Close()

	files, err := dir.Readdir(-1)
	if err != nil {
		logger.Errorf("Failed to read root directory: %v", err)
		return dirs
	}

	for _, file := range files {
		baseName := file.Name()
		realPath := utils.JoinPathAsUnix(s.idx.Path, baseName)
		indexPath := "/" + baseName + "/"
		if !file.IsDir() {
			continue
		}
		if omitList[baseName] {
			logger.Debugf("Skipping scanner creation for omitted directory: %s", realPath)
			continue
		}
		if !s.idx.shouldInclude(baseName) {
			logger.Debugf("Skipping scanner creation for non-included directory: %s", realPath)
			continue
		}
		hidden := IsHidden(realPath)
		isSymlink := file.Mode()&os.ModeSymlink != 0
		if s.idx.ShouldSkip(true, indexPath, hidden, isSymlink, true) {
			logger.Debugf("Skipping scanner creation for excluded directory: %s", realPath)
			continue
		}
		dirs = append(dirs, indexPath)
	}

	return dirs
}

// calculateSleepTime determines how long to wait before the next scan
func (s *Scanner) calculateSleepTime() time.Duration {
	sleepTime := scanSchedule[s.currentSchedule] + s.smartModifier
	if s.idx.Config.IndexingInterval > 0 {
		sleepTime = time.Duration(s.idx.Config.IndexingInterval) * time.Minute
	}

	return sleepTime
}

// updateSchedule adjusts the scanner's schedule based on whether files changed
func (s *Scanner) updateSchedule() {

	// Calculate sleep time BEFORE updating schedule (based on current/old schedule)
	s.nextSleepTime = scanSchedule[s.currentSchedule] + s.smartModifier
	if s.idx.Config.IndexingInterval > 0 {
		s.nextSleepTime = time.Duration(s.idx.Config.IndexingInterval) * time.Minute
	}

	// Adjust schedule based on file changes
	if s.filesChanged {
		logger.Debugf("Scanner [%s] detected changes, adjusting schedule", s.scanPath)
		// Move to at least schedule index 3 or reduce interval
		if s.currentSchedule > 3 {
			s.currentSchedule = 3
		} else if s.currentSchedule > 0 {
			s.currentSchedule--
		}
	} else {
		// Increment toward the longest interval if no changes
		if s.currentSchedule < len(scanSchedule)-1 {
			s.currentSchedule++
		}
	}

	// Cap simple complexity (1) at schedule 4
	// Don't apply this cap if complexity is still unknown (0)
	if s.complexity == 1 && s.currentSchedule > 4 {
		s.currentSchedule = 4
	}

	// Ensure currentSchedule stays within bounds
	if s.currentSchedule < 0 {
		s.currentSchedule = 0
	} else if s.currentSchedule >= len(scanSchedule) {
		s.currentSchedule = len(scanSchedule) - 1
	}

	// Persist index after schedule update (happens after each scan)
	if err := s.idx.Save(); err != nil {
		logger.Errorf("Failed to save index after schedule update: %v", err)
	}
}

// updateComplexity calculates the complexity level (1-10) for this scanner's directory
// 0: unknown
func (s *Scanner) updateComplexity() {
	s.complexity = calculateComplexity(s.fullScanTime, s.numDirs)
	// Set smartModifier based on complexity level
	if modifier, ok := complexityModifier[s.complexity]; ok {
		s.smartModifier = modifier
	} else {
		s.smartModifier = 0
	}
	// Persist index after complexity update (happens after full scans)
	if err := s.idx.Save(); err != nil {
		logger.Errorf("Failed to save index after complexity update: %v", err)
	}
}

// directoryExists checks if the scanner's directory still exists
func (s *Scanner) directoryExists() bool {
	realPath := strings.TrimRight(s.idx.Path, "/") + s.scanPath
	realPath = strings.TrimSuffix(realPath, "/")

	_, err := os.Stat(realPath)
	return err == nil
}

// removeSelf removes this scanner from the index's scanner map
func (s *Scanner) removeSelf() {
	s.idx.mu.Lock()
	defer s.idx.mu.Unlock()

	delete(s.idx.scanners, s.scanPath)
	logger.Infof("Removed scanner [%s] from active scanners", s.scanPath)
}

// stop gracefully stops the scanner
// Uses sync.Once to ensure the channel is only closed once, preventing panic from "close of closed channel"
func (s *Scanner) stop() {
	s.stopOnce.Do(func() {
		close(s.stopChan)
	})
}

func (s *Scanner) syncStatsWithDB() {
	var dirs, files uint64
	var err error

	if s.scanPath == "/" {
		// Root scanner never counts directories (those are handled by child scanners)
		dirs = 0
		files, err = s.idx.db.GetDirectFileCount(s.idx.Name, "/")
	} else {
		// Child scanner: count directories and files recursively
		dirs, files, err = s.idx.db.GetRecursiveCount(s.idx.Name, s.scanPath)
	}

	if err != nil {
		logger.Errorf("Failed to get count for %s: %v", s.scanPath, err)
		return
	}

	s.numDirs = dirs
	s.numFiles = files
}

// checkFolderModtime checks if a folder's modtime changed since last scan
// Returns (changed, error) where changed=true if modtime is after lastScanned, false otherwise
func (s *Scanner) checkFolderModtime(folderPath string) (bool, error) {
	realPath := strings.TrimRight(s.idx.Path, "/") + folderPath
	realPath = strings.TrimSuffix(realPath, "/")

	info, err := os.Stat(realPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Folder was deleted - will be caught by purge
			return false, nil
		}
		logger.Errorf("[QUICK_SCAN] Error stating %s: %v", folderPath, err)
		return false, err
	}

	// Compare folder modtime against when the last scan started
	// If modtime is after last scan, the folder has been modified
	modtimeChanged := info.ModTime().After(s.lastScanned)

	if modtimeChanged {
		// Modtime changed - read directory contents and update database
		s.filesChanged = true

		dir, err := os.Open(realPath)
		if err != nil {
			logger.Errorf("[QUICK_SCAN] Error opening %s: %v", folderPath, err)
			return true, err
		}
		defer dir.Close()

		opts := Options{
			Recursive:     false, // Don't recurse - we're checking each folder individually
			IsRoutineScan: true,
		}

		dirInfo, err := s.idx.GetDirInfo(dir, info, folderPath, opts, s)
		if err != nil {
			logger.Errorf("[QUICK_SCAN] Error getting dir info for %s: %v", folderPath, err)
			return true, err
		}

		// Update metadata (will be batched)
		s.idx.UpdateMetadata(dirInfo, s)

		// Track this directory as having been updated (for file purge logic)
		s.idx.mu.Lock()
		s.idx.scanUpdatedPaths[folderPath] = true
		s.idx.mu.Unlock()

		return true, nil
	}

	// Modtime unchanged - update last_updated to prevent folder from being purged
	// Note: we DON'T add this folder to scanUpdatedPaths, so Phase 2 won't check its files
	fileInfo := &iteminfo.FileInfo{
		Path: folderPath,
		ItemInfo: iteminfo.ItemInfo{
			Name:       filepath.Base(strings.TrimSuffix(folderPath, "/")),
			Size:       0, // Folder size is managed in-memory via folderSizes map
			ModTime:    info.ModTime(),
			Type:       "directory",
			Hidden:     false,
			HasPreview: false,
		},
	}

	// Add to batch for bulk update
	s.batchItems = append(s.batchItems, fileInfo)
	if len(s.batchItems) >= s.idx.db.BatchSize {
		s.flushBatch()
	}

	return false, nil
}

func (s *Scanner) purgeStaleEntries(isRoot bool, isQuickScan bool) {
	if s.scanStartTime == 0 {
		return
	}

	var deletedCount int
	var err error

	if isQuickScan {
		// Quick scan cleanup (two-phase):
		// Phase 1: Delete folders that weren't updated (they were deleted from filesystem)
		deletedCount, err = s.idx.db.DeleteStaleFolders(s.idx.Name, s.scanPath, s.scanStartTime, isRoot)
		if err != nil {
			logger.Errorf("[DB_MAINTENANCE] Failed to purge stale folders for %s: %v", s.scanPath, err)
			return
		}

		// Phase 2: Delete files in directories that had modtime changes
		// Collect list of updated directories from this scan
		var updatedDirs []string
		s.idx.mu.RLock()
		for path := range s.idx.scanUpdatedPaths {
			// Only include paths within this scanner's scope
			if s.scanPath == "/" {
				// Root scanner: only direct children (no nested paths)
				if strings.Count(path, "/") == 2 && strings.HasPrefix(path, "/") {
					updatedDirs = append(updatedDirs, path)
				}
			} else {
				// Child scanner: all paths under this scanner's path
				if strings.HasPrefix(path, s.scanPath) {
					updatedDirs = append(updatedDirs, path)
				}
			}
		}
		s.idx.mu.RUnlock()

		if len(updatedDirs) > 0 {
			deletedCount, err = s.idx.db.DeleteStaleFilesInDirs(s.idx.Name, updatedDirs, s.scanStartTime)
			if err != nil {
				logger.Errorf("[DB_MAINTENANCE] Failed to purge stale files in updated dirs for %s: %v", s.scanPath, err)
			}
		}

		err = nil // Reset err since we handled errors separately above
	} else {
		// Full scan cleanup: delete all items not updated in scanner scope
		deletedCount, err = s.idx.db.DeleteStaleEntries(s.idx.Name, s.scanPath, s.scanStartTime, isRoot)
	}

	if err != nil {
		logger.Errorf("[DB_MAINTENANCE] Failed to purge stale entries for %s: %v", s.scanPath, err)
		return
	}
	if deletedCount >= 10000 {
		logger.Warningf("[DB_MAINTENANCE] high purge count (%d) for %s", deletedCount, s.scanPath)
	}
}
