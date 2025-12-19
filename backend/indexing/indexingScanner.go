package indexing

import (
	"os"
	"strings"
	"time"

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
}

// start begins the scanner's main loop
func (s *Scanner) start() {
	// Do initial scan for all scanners
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
	// Mark that this scanner is attempting to scan (before acquiring mutex)
	// This prevents status from incorrectly showing "ready" while waiting for mutex
	s.idx.mu.Lock()
	s.isScanning = true
	s.idx.mu.Unlock()

	// Ensure isScanning is cleared even if we panic
	defer func() {
		s.idx.mu.Lock()
		s.isScanning = false
		s.idx.mu.Unlock()
		s.idx.aggregateStatsFromScanners() // Update status after clearing isScanning
	}()

	s.idx.scanMutex.Lock()

	// Mark which scanner is active (for status/logging)

	quick := s.fullScanCounter > 0 && s.fullScanCounter < 5
	s.fullScanCounter++
	if s.fullScanCounter >= 5 {
		s.fullScanCounter = 0
	}

	s.runIndexing(quick)

	// Update this scanner's schedule based on results
	s.updateSchedule()

	s.idx.mu.Lock()
	allIdle := true
	for _, scanner := range s.idx.scanners {
		if !scanner.lastScanned.IsZero() && time.Since(scanner.lastScanned) > 1*time.Minute {
			allIdle = false
			break
		}
	}
	s.idx.mu.Unlock()

	s.idx.scanMutex.Unlock()

	if allIdle {
		s.idx.updateRootDirectorySize()
	}
	// Note: aggregateStatsFromScanners() is called in defer to ensure isScanning is cleared first
}

// runIndexing performs the actual indexing work
func (s *Scanner) runIndexing(quick bool) {
	if s.scanPath == "/" {
		s.runRootScan(quick)
	} else {
		s.runChildScan(quick)
	}

	s.lastScanned = time.Now()
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
		s.idx.processedInodes = make(map[uint64]struct{})
		s.idx.FoundHardLinks = make(map[string]uint64)
		s.idx.mu.Unlock()
	}

	s.idx.mu.Lock()
	s.idx.batchItems = make([]*iteminfo.FileInfo, 0, 5000)
	s.idx.mu.Unlock()
	logger.Debugf("[MEMORY] Root scan started: batch buffer allocated (capacity: 5000 items)")

	s.filesChanged = false
	startTime := time.Now()

	_, _, err := s.idx.indexDirectory("/", config)
	if err != nil {
		logger.Errorf("Root scanner error: %v", err)
	}

	s.idx.flushBatch()

	if !quick {
		// Recalculate directory sizes after batch insertion
		_, err := s.idx.db.RecalculateDirectorySizes(s.idx.Name, "/")
		if err != nil {
			logger.Errorf("[SIZE_CALC] Failed to recalculate directory sizes: %v", err)
		}
		s.purgeStaleEntries()
		s.syncStatsWithDB()
	}

	// Note: Root directory size will be calculated by updateRootDirectorySize()
	// which sums all child directories + root files from the database
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
	s.checkForNewChildDirectories()
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
		s.idx.processedInodes = make(map[uint64]struct{})
		s.idx.FoundHardLinks = make(map[string]uint64)
		s.idx.mu.Unlock()
	}

	s.idx.mu.Lock()
	s.filesChanged = false
	startTime := time.Now()
	s.idx.batchItems = make([]*iteminfo.FileInfo, 0, 5000)
	s.idx.mu.Unlock()
	logger.Debugf("[MEMORY] Child scan started for %s: batch buffer allocated (capacity: 5000 items)", s.scanPath)

	_, _, err := s.idx.indexDirectory(s.scanPath, config)
	if err != nil {
		logger.Errorf("Scanner [%s] error: %v", s.scanPath, err)
	}

	s.idx.flushBatch()

	if !quick {
		// Recalculate directory sizes after batch insertion
		_, err := s.idx.db.RecalculateDirectorySizes(s.idx.Name, s.scanPath)
		if err != nil {
			logger.Errorf("[SIZE_CALC] Failed to recalculate directory sizes for %s: %v", s.scanPath, err)
		}

		s.purgeStaleEntries()
		s.syncStatsWithDB()
	}

	// Note: Directory size is calculated recursively by indexDirectory()
	// and stored in the database. Root directory size calculation happens
	// in updateRootDirectorySize() after this scan completes.

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

	// Check for deleted directories and stop their scanners
	for path, scanner := range existingScanners {
		if !currentDirsMap[path] {
			logger.Debugf("Directory [%s] no longer exists, stopping scanner", path)
			scanner.stop()
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
		if s.idx.shouldSkip(true, hidden, dirPath, baseName, actionConfig{}) {
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

	// Trigger stats aggregation to update overall index
	go s.idx.aggregateStatsFromScanners()
}

// stop gracefully stops the scanner
func (s *Scanner) stop() {
	close(s.stopChan)
}

func (s *Scanner) syncStatsWithDB() {
	if s.idx.db == nil {
		return
	}
	dirs, files, err := s.idx.db.GetRecursiveCount(s.idx.Name, s.scanPath)
	if err != nil {
		logger.Errorf("Failed to get recursive count for %s: %v", s.scanPath, err)
		return
	}

	s.numDirs = dirs
	s.numFiles = files
}

func (s *Scanner) purgeStaleEntries() {
	if s.idx.db == nil || s.scanStartTime == 0 {
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
}
