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

// Removed: Old single-scanner implementation - replaced by multi-scanner system in indexingScanner.go

// markFilesChanged marks that files have changed in the currently active scanner
func (idx *Index) markFilesChanged() {
	idx.mu.RLock()
	activePath := idx.activeScannerPath
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

// Removed: UpdateSchedule - now handled per-scanner in Scanner.updateSchedule()

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

// Removed: RunIndexing - replaced by multi-scanner system where each scanner handles its own indexing

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
