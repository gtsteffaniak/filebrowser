package indexing

import (
	"os"
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

	filesChanged  bool
	lastScanned   time.Time
	quickScanTime int
	fullScanTime  int
	scanStartTime int64
	isScanning    bool // True when scanner is actively scanning or waiting for mutex

	numDirs  uint64
	numFiles uint64

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

		// Calculate sleep based on this scanner's schedule
		sleepTime := s.calculateSleepTime()

		select {
		case <-s.stopChan:
			logger.Debugf("Scanner [%s] received stop signal", s.scanPath)
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
		s.idx.mu.Lock()
		s.isScanning = false
		s.idx.mu.Unlock()
		err := s.idx.PostScan()
		if err != nil {
			logger.Errorf("Failed to post scan: %v", err)
		}
	}()

	// Root scanner can run independently since it's non-recursive
	if s.scanPath != "/" {
		s.idx.childScanMutex.Lock()
		s.idx.mu.Lock()
		s.isScanning = true
		s.idx.mu.Unlock()
		defer s.idx.childScanMutex.Unlock()
	}

	// Flush any pending parent size updates before starting scan to ensure data consistency
	s.idx.FlushPendingParentSizeUpdates()

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
	startTime := time.Now()
	if s.scanPath == "/" {
		s.runRootScan(quick)
	} else {
		s.runChildScan(quick)
	}
	logger.Debugf("[%s] Scan for %s completed in %d seconds", s.idx.Name, s.scanPath, int(time.Since(startTime).Seconds()))
}

func (s *Scanner) runRootScan(quick bool) {
	config := actionConfig{
		Quick:         quick,
		Recursive:     false,
		IsRoutineScan: true,
	}

	if !quick {
		s.numDirs = 0
		s.numFiles = 0
		s.scanStartTime = time.Now().Unix()
		s.idx.mu.Lock()
		// Set scan session start time if this is the first full scan in the session
		if s.idx.scanSessionStartTime == 0 {
			s.idx.scanSessionStartTime = s.scanStartTime
		}
		s.idx.processedInodes = make(map[uint64]struct{})
		s.idx.FoundHardLinks = make(map[string]uint64)
		s.idx.mu.Unlock()
	}

	s.idx.mu.Lock()
	batchSize := s.idx.db.BatchSize
	s.idx.batchItems = make([]*iteminfo.FileInfo, 0, batchSize)
	s.idx.mu.Unlock()

	s.filesChanged = false
	startTime := time.Now()

	_, _, err := s.idx.indexDirectory("/", config)
	if err != nil {
		logger.Errorf("Root scanner error: %v", err)
	}

	s.idx.flushBatch()

	if !quick {
		s.syncStatsWithDB()
	}
	scanDuration := int(time.Since(startTime).Seconds())

	if quick {
		s.quickScanTime = scanDuration
		// Sync stats with DB after quick scan to ensure accurate counts
		// Quick scans increment counters but don't reset them, so we need to sync
		s.syncStatsWithDB()
	} else {
		s.fullScanTime = scanDuration
		s.updateComplexity()
	}
	s.lastScanned = time.Now()
	s.checkForNewChildDirectories()
	if err := s.idx.db.ShrinkMemory(); err != nil {
		logger.Errorf("Failed to shrink memory: %v", err)
	}
}

func (s *Scanner) runChildScan(quick bool) {
	config := actionConfig{
		Quick:         quick,
		Recursive:     true,
		IsRoutineScan: true,
	}

	if !quick {
		s.numDirs = 0
		s.numFiles = 0
		s.scanStartTime = time.Now().Unix()
		s.idx.mu.Lock()
		// Set scan session start time if this is the first full scan in the session
		if s.idx.scanSessionStartTime == 0 {
			s.idx.scanSessionStartTime = s.scanStartTime
		}
		s.idx.processedInodes = make(map[uint64]struct{})
		s.idx.FoundHardLinks = make(map[string]uint64)
		s.idx.mu.Unlock()
	}

	s.idx.mu.Lock()
	s.filesChanged = false
	startTime := time.Now()
	batchSize := s.idx.db.BatchSize
	s.idx.batchItems = make([]*iteminfo.FileInfo, 0, batchSize)
	s.idx.mu.Unlock()

	// indexDirectory returns the total size of this directory (including all subdirectories)
	// For child scanners, we need to count the directory itself (1) plus any subdirectories
	// The recursive scan counts subdirectories, but not the directory being scanned itself
	// If a scanner exists, it means it passed all exclusion checks, so it should count itself
	dirSize, _, err := s.idx.indexDirectory(s.scanPath, config)
	if err != nil {
		logger.Errorf("Scanner [%s] error: %v", s.scanPath, err)
	} else if config.Recursive && config.IsRoutineScan {
		// Count the directory itself (each child scanner counts itself as 1 directory)
		// Subdirectories are already counted by the recursive scan in indexDirectory
		s.idx.mu.Lock()
		// Use unlocked version since we already hold the write lock
		s.idx.incrementScannerDirsUnlocked()
		s.idx.mu.Unlock()
	}

	s.idx.flushBatch()

	if !quick {
		// Each scanner is responsible for updating its own directory size
		// This uses in-memory calculation during recursive traversal to avoid expensive SQL queries
		// Normalize path to match database format (with trailing slash for directories)
		normalizedPath := utils.AddTrailingSlashIfNotExists(s.scanPath)

		// Check if this directory was updated by the scan itself
		// If so, always update (it's part of the scan). If not, use timestamp checking
		// to avoid overwriting API updates that occurred during scan
		s.idx.mu.RLock()
		wasUpdatedByScan := s.idx.scanUpdatedPaths[normalizedPath]
		scanStartTime := s.scanStartTime
		s.idx.mu.RUnlock()

		if wasUpdatedByScan {
			// Directory was updated by the scan - always update size (no timestamp check)
			if err := s.idx.db.UpdateDirectorySize(s.idx.Name, normalizedPath, dirSize); err != nil {
				logger.Errorf("[SIZE_CALC] Failed to update directory size for %s: %v", normalizedPath, err)
			}
			// No need to log success - this is expected during scans
		} else {
			// Directory existed before scan - use timestamp checking to avoid overwriting API updates
			_, err := s.idx.db.UpdateDirectorySizeIfStale(s.idx.Name, normalizedPath, dirSize, scanStartTime)
			if err != nil {
				logger.Errorf("[SIZE_CALC] Failed to update directory size for %s: %v", normalizedPath, err)
			}
			// No need to log success - this is expected during scans
		}

		s.purgeStaleEntries()
		s.syncStatsWithDB()
	}

	scanDuration := int(time.Since(startTime).Seconds())

	if quick {
		s.quickScanTime = scanDuration
		// Sync stats with DB after quick scan to ensure accurate counts
		// Quick scans increment counters but don't reset them, so we need to sync
		s.syncStatsWithDB()
	} else {
		s.fullScanTime = scanDuration
		s.updateComplexity()
	}
	s.lastScanned = time.Now()

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
	realPath := strings.TrimRight(s.idx.Path, "/") + "/"

	dir, err := os.Open(realPath)
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
		if !file.IsDir() {
			continue
		}
		dirPath := "/" + baseName + "/"
		if omitList[baseName] {
			logger.Debugf("Skipping scanner creation for omitted directory: %s", dirPath)
			continue
		}
		if !s.idx.shouldInclude(baseName) {
			logger.Debugf("Skipping scanner creation for non-included directory: %s", dirPath)
			continue
		}
		hidden := isHidden(file, s.idx.Path+dirPath)
		// Use IsRoutineScan: true to ensure NeverWatchPaths is checked
		config := actionConfig{IsRoutineScan: true}
		if s.idx.shouldSkip(true, hidden, dirPath, baseName, config) {
			logger.Debugf("Skipping scanner creation for excluded directory: %s", dirPath)
			continue
		}
		dirs = append(dirs, dirPath)
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
	// Adjust schedule based on file changes
	if s.filesChanged {
		logger.Debugf("Scanner [%s] detected changes, adjusting schedule", s.scanPath)
		// Move to at least the full-scan anchor or reduce interval
		if s.currentSchedule > fullScanAnchor {
			s.currentSchedule = fullScanAnchor
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

func (s *Scanner) purgeStaleEntries() {
	if s.scanStartTime == 0 {
		return
	}
	deletedCount, err := s.idx.db.DeleteStaleEntries(s.idx.Name, s.scanPath, s.scanStartTime)
	if err != nil {
		logger.Errorf("[DB_MAINTENANCE] Failed to purge stale entries for %s: %v", s.scanPath, err)
		return
	}
	if deletedCount > 0 {
		logger.Debugf("[DB_MAINTENANCE] Purged %d stale entries for scan path: %s", deletedCount, s.scanPath)
	}
	// Log warning if unexpectedly high number of deletions (may indicate a problem)
	if deletedCount > 10000 {
		logger.Warningf("[DB_MAINTENANCE] high purge count (%d) for %s", deletedCount, s.scanPath)
	}
}
