package indexing

import (
	"os"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

// Each scanner has its own schedule and stats, but only one can run at a time
type Scanner struct {
	// Identity
	scanPath string // "/" for root scanner, "/Documents/" for child scanners

	currentSchedule int
	smartModifier   time.Duration
	complexity      uint // 0-10 scale: 0=unknown, 1=simple, 2-6=normal, 7-9=complex, 10=highlyComplex
	fullScanCounter int  // every 5th scan is a full scan

	filesChanged  bool
	lastScanned   time.Time
	quickScanTime int
	fullScanTime  int

	// size tracking
	numDirs          uint64 // Local count for this path
	numFiles         uint64 // Local count for this path
	size             uint64 // Size contributed by this scanner (for delta calculation)
	previousNumDirs  uint64 // Previous numDirs value (preserved across scans)
	previousNumFiles uint64 // Previous numFiles value (preserved across scans)
	previousSize     uint64 // Previous size value (preserved across scans)

	// Reference back to parent index
	idx *Index

	// Control
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
	s.idx.scanMutex.Lock()

	// Mark which scanner is active (for status/logging)
	s.idx.mu.Lock()
	s.idx.activeScannerPath = s.scanPath
	s.idx.mu.Unlock()

	quick := s.fullScanCounter > 0 && s.fullScanCounter < 5
	s.fullScanCounter++
	if s.fullScanCounter >= 5 {
		s.fullScanCounter = 0
	}

	s.runIndexing(quick)

	// Update this scanner's schedule based on results
	s.updateSchedule()

	// If this is the root scanner, update the last root scan time
	if s.scanPath == "/" {
		s.idx.mu.Lock()
		s.idx.lastRootScanTime = time.Now()
		s.idx.mu.Unlock()
	}

	// Clear active scanner
	s.idx.mu.Lock()
	s.idx.activeScannerPath = ""
	s.idx.mu.Unlock()

	s.idx.scanMutex.Unlock()

	// Aggregate stats to Index level and update status
	s.idx.aggregateStatsFromScanners()
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

// runRootScan scans only the root directory (non-recursive) and checks for new child directories
func (s *Scanner) runRootScan(quick bool) {
	config := actionConfig{
		Quick:         quick,
		Recursive:     false,
		IsRoutineScan: true,
	}

	// Store previous values before scanning (preserved across scans)
	prevNumDirs := s.previousNumDirs
	prevNumFiles := s.previousNumFiles
	prevSize := s.previousSize

	// Reset counters for full scan (they will be incremented during indexing)
	if !quick {
		s.numDirs = 0
		s.numFiles = 0
	}

	s.filesChanged = false
	startTime := time.Now()

	err := s.idx.indexDirectory("/", config)
	if err != nil {
		logger.Errorf("Root scanner error: %v", err)
	}

	// Root scanner gets values directly from index metadata - simple!
	s.idx.mu.RLock()
	rootDirInfo, exists := s.idx.Directories["/"]
	s.idx.mu.RUnlock()

	newNumDirs := prevNumDirs
	newNumFiles := prevNumFiles
	newsize := prevSize
	if exists && rootDirInfo != nil {
		for _, file := range rootDirInfo.Files {
			newsize += uint64(file.Size)
			newNumFiles++
		}
		for range rootDirInfo.Folders {
			newNumDirs++
		}
	}
	// Update scanner with new values
	s.size = newsize

	// Update previous values for next scan (preserve history - don't reset on new scans)
	s.previousNumDirs = newNumDirs
	s.previousNumFiles = newNumFiles
	s.previousSize = newsize
	scanDuration := int(time.Since(startTime).Seconds())
	if quick {
		s.quickScanTime = scanDuration
	} else {
		s.fullScanTime = scanDuration
		s.updateComplexity()
	}
	s.checkForNewChildDirectories()
}

// runChildScan scans a specific directory recursively
func (s *Scanner) runChildScan(quick bool) {
	config := actionConfig{
		Quick:         quick,
		Recursive:     true,
		IsRoutineScan: true,
	}

	// Store previous values before scanning (preserved across scans)
	prevSize := s.previousSize

	// Reset counters for full scan (they will be incremented during indexing)
	if !quick {
		s.numDirs = 0
		s.numFiles = 0
	}

	s.filesChanged = false
	startTime := time.Now()

	err := s.idx.indexDirectory(s.scanPath, config)
	if err != nil {
		logger.Errorf("Scanner [%s] error: %v", s.scanPath, err)
	}

	// Calculate new values after scan
	newNumDirs := s.numDirs
	newNumFiles := s.numFiles

	// For child scanners, calculate size from the directory info
	// Since child scanners don't modify totalSize, we get it from the directory metadata
	s.idx.mu.RLock()
	dirInfo, exists := s.idx.Directories[s.scanPath]
	s.idx.mu.RUnlock()

	newsize := prevSize
	if exists && dirInfo != nil {
		newsize = uint64(dirInfo.Size)
	}

	// Update scanner with new values
	s.size = newsize

	// Update previous values for next scan (preserve history - don't reset on new scans)
	s.previousNumDirs = newNumDirs
	s.previousNumFiles = newNumFiles
	s.previousSize = newsize

	scanDuration := int(time.Since(startTime).Seconds())
	if quick {
		s.quickScanTime = scanDuration
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
