package indexing

import (
	"encoding/json"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/events"
	"github.com/gtsteffaniak/go-logger/logger"
)

// schedule in minutes
var scanSchedule = map[int]time.Duration{
	0: 5 * time.Minute, // 5 minute quick scan & 25 minutes for a full scan
	1: 10 * time.Minute,
	2: 20 * time.Minute,
	3: 40 * time.Minute, // reset anchor for full scan
	4: 1 * time.Hour,
	5: 2 * time.Hour,
	6: 3 * time.Hour, // [6]
	7: 4 * time.Hour, // [7] 4 hours for quick scan & 20 hours for a full scan
	8: 8 * time.Hour,
	9: 12 * time.Hour,
}

var fullScanAnchor = 3 // index of the schedule for a full scan

func (idx *Index) newScanner(origin string) {
	fullScanCounter := 0 // every 5th scan is a full scan
	for {
		// Determine sleep time with modifiers
		fullScanCounter++
		sleepTime := scanSchedule[idx.CurrentSchedule] + idx.SmartModifier
		if idx.Assessment == "simple" {
			sleepTime = scanSchedule[idx.CurrentSchedule] - idx.SmartModifier
		}
		if idx.Config.IndexingInterval > 0 {
			sleepTime = time.Duration(idx.Config.IndexingInterval) * time.Minute
		}
		// Log and sleep before indexing
		logger.Debugf("Next scan in %v", sleepTime)
		time.Sleep(sleepTime)
		if fullScanCounter == 5 {
			idx.RunIndexing(origin, false) // Full scan
			fullScanCounter = 0
		} else {
			idx.RunIndexing(origin, true) // Quick scan
		}
		idx.UpdateSchedule()
	}
}

// markFilesChanged marks that files have changed in the currently active scanner
func (idx *Index) markFilesChanged() {
	idx.mu.RLock()
	activePath := idx.activeScannerPath
	idx.mu.RUnlock()

	if activePath == "" {
		// Legacy mode or no active scanner
		idx.mu.Lock()
		idx.FilesChangedDuringIndexing = true
		idx.mu.Unlock()
		return
	}

	idx.mu.RLock()
	scanner, exists := idx.scanners[activePath]
	idx.mu.RUnlock()

	if exists {
		scanner.filesChanged = true
	}
}

// incrementScannerDirs increments the directory counter for the active scanner
func (idx *Index) incrementScannerDirs() {
	idx.mu.RLock()
	activePath := idx.activeScannerPath
	scanner, exists := idx.scanners[activePath]
	idx.mu.RUnlock()

	if exists {
		scanner.numDirs++
	}
}

// incrementScannerFiles increments the file counter for the active scanner
func (idx *Index) incrementScannerFiles() {
	idx.mu.RLock()
	activePath := idx.activeScannerPath
	scanner, exists := idx.scanners[activePath]
	idx.mu.RUnlock()

	if exists {
		scanner.numFiles++
	}
}

func (idx *Index) PreScan() error {
	return idx.SetStatus(INDEXING)
}

func (idx *Index) PostScan() error {
	idx.mu.Lock()
	idx.garbageCollection()
	idx.wasIndexed = true
	idx.runningScannerCount--
	idx.mu.Unlock()
	if idx.runningScannerCount == 0 {
		return idx.SetStatus(READY)
	}
	return nil
}

func (idx *Index) garbageCollection() {
	for path := range idx.Directories {
		_, ok := idx.DirectoriesLedger[path]
		if !ok {
			idx.Directories[path] = nil
			delete(idx.Directories, path)
			idx.NumDeleted++
		}
	}
	// Reset the ledger for the next scan.
	idx.DirectoriesLedger = make(map[string]struct{})
}

func (idx *Index) UpdateSchedule() {
	// Adjust schedule based on file changes
	if idx.FilesChangedDuringIndexing {
		logger.Debugf("Files changed during indexing [%v], adjusting schedule.", idx.Name)
		// Move to at least the full-scan anchor or reduce interval
		if idx.CurrentSchedule > fullScanAnchor {
			idx.CurrentSchedule = fullScanAnchor
		} else if idx.CurrentSchedule > 0 {
			idx.CurrentSchedule--
		}
	} else {
		// Increment toward the longest interval if no changes
		if idx.CurrentSchedule < len(scanSchedule)-1 {
			idx.CurrentSchedule++
		}
	}
	if idx.Assessment == "simple" && idx.CurrentSchedule > 3 {
		idx.CurrentSchedule = 3
	}
	// Ensure `currentSchedule` stays within bounds
	if idx.CurrentSchedule < 0 {
		idx.CurrentSchedule = 0
	} else if idx.CurrentSchedule >= len(scanSchedule) {
		idx.CurrentSchedule = len(scanSchedule) - 1
	}
}

func (idx *Index) SendSourceUpdateEvent() error {
	if idx.mock {
		logger.Debug("Skipping source update event for mock index.")
		return nil
	}
	reducedIndex, err := GetIndexInfo(idx.Name)
	if err != nil {
		return err
	}
	sourceAsMap := map[string]ReducedIndex{
		idx.Name: reducedIndex,
	}
	message, err := json.Marshal(sourceAsMap)
	if err != nil {
		return err
	}
	events.SendSourceUpdate(idx.Name, string(message))
	return nil
}

// RunIndexing is the legacy indexing method, kept for compatibility
// Now primarily used for initial setup before multi-scanner starts
func (idx *Index) RunIndexing(origin string, quick bool) {
	if idx.runningScannerCount > 0 {
		logger.Debugf("Indexing already in progress for [%v]", idx.Name)
		return
	}
	err := idx.PreScan()
	if err != nil {
		logger.Errorf("Error during indexing: %v", err)
		return
	}

	prevNumDirs := idx.NumDirs
	prevNumFiles := idx.NumFiles
	if quick {
		logger.Debugf("Starting quick scan for [%v]", idx.Name)
	} else {
		logger.Debugf("Starting full scan for [%v]", idx.Name)
		idx.mu.Lock()
		idx.NumDirs = 0
		idx.NumFiles = 0
		idx.processedInodes = make(map[uint64]struct{})
		idx.FoundHardLinks = make(map[string]uint64)
		idx.mu.Unlock()
	}
	startTime := time.Now()
	idx.FilesChangedDuringIndexing = false
	// Perform the indexing operation
	config := actionConfig{
		Quick:         quick,
		Recursive:     true,
		IsRoutineScan: idx.wasIndexed, // This is a routine scan if we already have an index
	}
	err = idx.indexDirectory("/", config)
	if err != nil {
		logger.Errorf("Error during indexing: %v", err)
	}
	firstRun := time.Time.Equal(idx.LastIndexed, time.Time{})
	// Update the LastIndexed time
	idx.LastIndexed = time.Now()
	idx.LastIndexedUnix = idx.LastIndexed.Unix()
	if quick {
		idx.QuickScanTime = int(time.Since(startTime).Seconds())
		logger.Debugf("Time spent indexing [%v]: %v seconds", idx.Name, idx.QuickScanTime)
	} else {
		idx.FullScanTime = int(time.Since(startTime).Seconds())
		// update smart indexing
		if idx.FullScanTime < 3 || idx.NumDirs < 10000 {
			idx.Assessment = "simple"
			idx.SmartModifier = 4 * time.Minute
		} else if idx.FullScanTime > 120 || idx.NumDirs > 500000 {
			idx.Assessment = "complex"
			modifier := idx.FullScanTime / 10 // seconds
			idx.SmartModifier = time.Duration(modifier) * time.Minute
		} else {
			idx.Assessment = "normal"
		}
		if firstRun {
			logger.Infof("Index assessment         : [%v] complexity=%v directories=%v files=%v", idx.Name, idx.Assessment, idx.NumDirs, idx.NumFiles)
		} else {
			logger.Debugf("Index assessment         : [%v] complexity=%v directories=%v files=%v", idx.Name, idx.Assessment, idx.NumDirs, idx.NumFiles)
		}
		if idx.NumDirs != prevNumDirs || idx.NumFiles != prevNumFiles {
			idx.FilesChangedDuringIndexing = true
		}
		logger.Debugf("Time spent indexing [%v]: %v seconds", idx.Name, idx.FullScanTime)
	}

	err = idx.PostScan()
	if err != nil {
		logger.Errorf("Error during post scan indexing: %v", err)
		return
	}
}

// setupMultiScanner creates and starts the multi-scanner system
// Creates a root scanner (non-recursive) and child scanners for each top-level directory
func (idx *Index) setupMultiScanner() {
	logger.Infof("Setting up multi-scanner system for [%v]", idx.Name)

	idx.mu.Lock()
	idx.scanners = make(map[string]*Scanner)
	idx.mu.Unlock()

	// Create and start root scanner
	rootScanner := idx.createRootScanner()
	idx.mu.Lock()
	idx.scanners["/"] = rootScanner
	idx.mu.Unlock()
	go rootScanner.start()

	// Wait a moment for root scanner to do initial scan and discover directories
	time.Sleep(3 * time.Second)

	// Discover existing top-level directories
	topLevelDirs := rootScanner.getTopLevelDirs()

	// Create child scanner for each top-level directory
	for _, dirPath := range topLevelDirs {
		childScanner := idx.createChildScanner(dirPath)

		idx.mu.Lock()
		idx.scanners[dirPath] = childScanner
		idx.mu.Unlock()

		go childScanner.start()
	}

	logger.Debugf("Created %d scanners for [%v] (1 root + %d children)", len(topLevelDirs)+1, idx.Name, len(topLevelDirs))
}

// createRootScanner creates a scanner for the root directory (non-recursive)
func (idx *Index) createRootScanner() *Scanner {
	return &Scanner{
		scanPath:        "/",
		isRoot:          true,
		idx:             idx,
		stopChan:        make(chan struct{}),
		currentSchedule: 0,
		fullScanCounter: 0,
		assessment:      "unknown",
	}
}

// createChildScanner creates a scanner for a specific child directory (recursive)
func (idx *Index) createChildScanner(dirPath string) *Scanner {
	return &Scanner{
		scanPath:        dirPath,
		isRoot:          false,
		idx:             idx,
		stopChan:        make(chan struct{}),
		currentSchedule: 0,
		fullScanCounter: 0,
		assessment:      "unknown",
	}
}

// Legacy function kept for backwards compatibility - now deprecated
func (idx *Index) setupIndexingScanners() {
	// Use new multi-scanner system
	idx.setupMultiScanner()
}

// GetScannerStatus returns detailed information about all active scanners
func (idx *Index) GetScannerStatus() map[string]interface{} {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	status := make(map[string]interface{})

	// Current active scanner (if any is running)
	status["activeScanner"] = idx.activeScannerPath
	status["isScanning"] = idx.activeScannerPath != ""

	// Individual scanner stats
	scannerStats := make([]map[string]interface{}, 0, len(idx.scanners))
	for path, scanner := range idx.scanners {
		scannerInfo := map[string]interface{}{
			"path":            path,
			"isRoot":          scanner.isRoot,
			"lastScanned":     scanner.lastScanned.Format(time.RFC3339),
			"assessment":      scanner.assessment,
			"currentSchedule": scanner.currentSchedule,
			"quickScanTime":   scanner.quickScanTime,
			"fullScanTime":    scanner.fullScanTime,
			"numDirs":         scanner.numDirs,
			"numFiles":        scanner.numFiles,
			"filesChanged":    scanner.filesChanged,
			"smartModifier":   scanner.smartModifier.String(),
		}
		scannerStats = append(scannerStats, scannerInfo)
	}
	status["scanners"] = scannerStats
	status["totalScanners"] = len(idx.scanners)

	return status
}
