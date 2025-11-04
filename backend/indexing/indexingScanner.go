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
	isRoot   bool   // true if this is the root scanner (non-recursive)

	// Per-scanner scheduling (not shared between scanners)
	currentSchedule int
	smartModifier   time.Duration
	assessment      string // "simple", "normal", "complex"
	fullScanCounter int    // every 5th scan is a full scan

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

// start begins the scanner's main loop
func (s *Scanner) start() {
	// Initial scan for child scanners (root scanner will trigger this through setupMultiScanner)
	if !s.isRoot {
		// Wait a bit to stagger initial scans
		time.Sleep(2 * time.Second)
		s.tryAcquireAndScan()
	}

	for {
		// Calculate sleep based on this scanner's schedule
		sleepTime := s.calculateSleepTime()

		select {
		case <-s.stopChan:
			return

		case <-time.After(sleepTime):
			// Time to scan! But must acquire mutex first
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

	// Determine if quick or full scan
	quick := s.fullScanCounter < 5
	s.fullScanCounter++
	if s.fullScanCounter >= 5 {
		s.fullScanCounter = 0
	}
	s.runIndexing(quick)

	// Update this scanner's schedule based on results
	s.updateSchedule()

	// Clear active scanner
	s.idx.mu.Lock()
	s.idx.activeScannerPath = ""
	s.idx.mu.Unlock()

	s.idx.scanMutex.Unlock()
}

// runIndexing performs the actual indexing work
func (s *Scanner) runIndexing(quick bool) {
	if s.isRoot {
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
		Recursive:     false, // 🔑 KEY: Don't recurse into child directories
		IsRoutineScan: s.idx.wasIndexed,
	}

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
	}

	// Check for new top-level directories and create scanners for them
	s.checkForNewChildDirectories()
}

// runChildScan scans a specific directory recursively
func (s *Scanner) runChildScan(quick bool) {
	config := actionConfig{
		Quick:         quick,
		Recursive:     true, // 🔑 Full recursive scan
		IsRoutineScan: s.idx.wasIndexed,
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
		s.updateAssessment()
	}
}

// checkForNewChildDirectories detects new top-level directories and creates scanners for them
func (s *Scanner) checkForNewChildDirectories() {
	if !s.isRoot {
		return
	}

	// Get current top-level directories from filesystem (already filtered by exclusion rules)
	currentDirs := s.getTopLevelDirs()

	// Check which scanners already exist
	s.idx.mu.RLock()
	existingScanners := make(map[string]bool)
	for path := range s.idx.scanners {
		existingScanners[path] = true
	}
	s.idx.mu.RUnlock()

	// Create scanner for any new directories (getTopLevelDirs already filtered excluded dirs)
	for _, dirPath := range currentDirs {
		if !existingScanners[dirPath] && dirPath != "/" {
			logger.Infof("Detected new directory, creating scanner: [%s]", dirPath)
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
		if s.idx.shouldSkip(true, hidden, dirPath, baseName, actionConfig{
			Quick:         false,
			Recursive:     true,
			IsRoutineScan: false,
		}) {
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
	if s.assessment == "simple" {
		sleepTime = scanSchedule[s.currentSchedule] - s.smartModifier
	}

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

	// Cap simple assessments at schedule 3
	if s.assessment == "simple" && s.currentSchedule > 3 {
		s.currentSchedule = 3
	}

	// Ensure currentSchedule stays within bounds
	if s.currentSchedule < 0 {
		s.currentSchedule = 0
	} else if s.currentSchedule >= len(scanSchedule) {
		s.currentSchedule = len(scanSchedule) - 1
	}
}

// updateAssessment calculates the complexity assessment for this scanner's directory
func (s *Scanner) updateAssessment() {
	if s.fullScanTime < 3 || s.numDirs < 10000 {
		s.assessment = "simple"
		s.smartModifier = 4 * time.Minute
	} else if s.fullScanTime > 120 || s.numDirs > 500000 {
		s.assessment = "complex"
		modifier := s.fullScanTime / 10 // seconds
		s.smartModifier = time.Duration(modifier) * time.Minute
	} else {
		s.assessment = "normal"
		s.smartModifier = 0
	}

	logger.Debugf("Scanner [%s] assessment: complexity=%v dirs=%v files=%v modifier=%v",
		s.scanPath, s.assessment, s.numDirs, s.numFiles, s.smartModifier)
}
