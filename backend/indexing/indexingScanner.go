package indexing

import (
	"os"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

// Scanner represents an independent scanner for a specific directory path
// Each scanner has its own schedule and stats, but only one can run at a time (protected by Index.scanMutex)
type Scanner struct {
	// Identity
	scanPath string // "/" for root scanner, "/Documents/" for child scanners

	// Per-scanner scheduling (not shared between scanners)
	currentSchedule int
	smartModifier   time.Duration
	complexity      uint // 0-10 scale: 0=unknown, 1=simple, 2-6=normal, 7-9=complex, 10=highlyComplex
	fullScanCounter int  // every 5th scan is a full scan

	// Per-scanner stats (not shared)
	filesChanged  bool
	lastScanned   time.Time
	quickScanTime int
	fullScanTime  int
	numDirs       uint64 // Local count for this path
	numFiles      uint64 // Local count for this path

	// Reference back to parent index
	idx *Index

	// Control
	stopChan chan struct{}
}

// calculateTimeScore returns a 1-10 score based on full scan time
func (s *Scanner) calculateTimeScore() uint {
	if s.fullScanTime == 0 {
		return 1 // No data yet, assume simple
	}
	// Time-based thresholds (in seconds)
	switch {
	case s.fullScanTime < 2:
		return 1
	case s.fullScanTime < 5:
		return 2
	case s.fullScanTime < 10:
		return 3
	case s.fullScanTime < 15:
		return 4
	case s.fullScanTime < 30:
		return 5
	case s.fullScanTime < 60:
		return 6
	case s.fullScanTime < 90:
		return 7
	case s.fullScanTime < 120:
		return 8
	case s.fullScanTime < 180:
		return 9
	default:
		return 10
	}
}

// calculateDirScore returns a 1-10 score based on directory count
func (s *Scanner) calculateDirScore() uint {
	// Directory-based thresholds
	switch {
	case s.numDirs < 2500:
		return 1
	case s.numDirs < 5000:
		return 2
	case s.numDirs < 10000:
		return 3
	case s.numDirs < 25000:
		return 4
	case s.numDirs < 50000:
		return 5
	case s.numDirs < 100000:
		return 6
	case s.numDirs < 250000:
		return 7
	case s.numDirs < 500000:
		return 8
	case s.numDirs < 1000000:
		return 9
	default:
		return 10
	}
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
			// Time to scan! But must acquire mutex first
			s.tryAcquireAndScan()
		}
	}
}

// tryAcquireAndScan attempts to acquire the global scan mutex and run a scan
func (s *Scanner) tryAcquireAndScan() {
	// Child scanners must wait for root scanner to go first each round
	if s.scanPath != "/" {
		s.idx.mu.RLock()
		lastRootScan := s.idx.lastRootScanTime
		myLastScan := s.lastScanned
		s.idx.mu.RUnlock()

		// If we've scanned more recently than the root, skip this cycle
		if !myLastScan.IsZero() && myLastScan.After(lastRootScan) {
			return
		}
	}

	s.idx.scanMutex.Lock()

	// Mark which scanner is active (for status/logging)
	s.idx.mu.Lock()
	s.idx.activeScannerPath = s.scanPath
	s.idx.mu.Unlock()

	// Determine if quick or full scan
	// First scan (fullScanCounter=0) is always full
	// Scans 1-4 are quick, scan 5 is full, then repeat
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

	// Aggregate stats to Index level and update status (after releasing active scanner)
	s.idx.aggregateStatsFromScanners()
}

// runIndexing performs the actual indexing work
func (s *Scanner) runIndexing(quick bool) {
	if s.scanPath == "/" {
		// ROOT SCANNER: Non-recursive, just scan root directory itself
		s.runRootScan(quick)
	} else {
		// CHILD SCANNER: Recursive scan of assigned directory
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

	// Reset counters for full scan
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

	scanDuration := int(time.Since(startTime).Seconds())
	if quick {
		s.quickScanTime = scanDuration
	} else {
		s.fullScanTime = scanDuration
		s.updateComplexity()
	}

	// Check for new top-level directories and create scanners for them
	s.checkForNewChildDirectories()
}

// runChildScan scans a specific directory recursively
func (s *Scanner) runChildScan(quick bool) {
	config := actionConfig{
		Quick:         quick,
		Recursive:     true,
		IsRoutineScan: true,
	}

	// Reset counters for full scan
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

		// Skip files - only create scanners for directories
		if !file.IsDir() {
			// Note: includeRootItems may contain files, but we only create scanners for directories
			continue
		}

		dirPath := "/" + baseName + "/"

		// Skip directories in omit list
		if omitList[baseName] {
			logger.Debugf("Skipping scanner creation for omitted directory: %s", dirPath)
			continue
		}

		// Check if we should include this directory (respects includeRootItems filter)
		// When includeRootItems is set, ONLY those items (that are directories) get scanners
		if !s.idx.shouldInclude(baseName) {
			logger.Debugf("Skipping scanner creation for non-included directory: %s", dirPath)
			continue
		}

		// Check if this directory should be excluded from indexing (respects exclusion rules)
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
	// Get base schedule time and apply complexity modifier
	sleepTime := scanSchedule[s.currentSchedule] + s.smartModifier

	// Allow manual override via config
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
// 0: unknown, 1: simple, 2-6: normal, 7-9: complex, 10: highlyComplex
func (s *Scanner) updateComplexity() {
	// Calculate complexity based on both scan time and directory count
	timeScore := s.calculateTimeScore()
	dirScore := s.calculateDirScore()

	// Use the higher score (more conservative approach)
	complexity := timeScore
	if dirScore > timeScore {
		complexity = dirScore
	}

	s.complexity = complexity

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
