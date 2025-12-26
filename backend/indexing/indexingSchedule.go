package indexing

import (
	"encoding/json"
	"strconv"
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

// complexityModifier defines time adjustments based on complexity level (0-10)
var complexityModifier = map[uint]time.Duration{
	0:  0 * time.Minute,  // unknown: no modifier
	1:  -4 * time.Minute, // simple: scan more frequently
	2:  -2 * time.Minute, // normal (lightest)
	3:  -1 * time.Minute,
	4:  0 * time.Minute, // baseline normal
	5:  1 * time.Minute,
	6:  2 * time.Minute, // normal (heaviest)
	7:  4 * time.Minute, // complex (lightest)
	8:  8 * time.Minute,
	9:  12 * time.Minute,
	10: 16 * time.Minute, // highlyComplex: scan less frequently
}

// calculateTimeScore returns a 1-10 score based on full scan time
func calculateTimeScore(fullScanTime int) uint {
	if fullScanTime == 0 {
		return 1 // No data yet, assume simple
	}
	switch {
	case fullScanTime < 2:
		return 1
	case fullScanTime < 5:
		return 2
	case fullScanTime < 15:
		return 3
	case fullScanTime < 30:
		return 4
	case fullScanTime < 60:
		return 5
	case fullScanTime < 90:
		return 6
	case fullScanTime < 120:
		return 7
	case fullScanTime < 180:
		return 8
	case fullScanTime < 300:
		return 9
	default:
		return 10
	}
}

// calculateDirScore returns a 1-10 score based on directory count
func calculateDirScore(numDirs uint64) uint {
	// Directory-based thresholds
	switch {
	case numDirs < 2500:
		return 1
	case numDirs < 5000:
		return 2
	case numDirs < 10000:
		return 3
	case numDirs < 25000:
		return 4
	case numDirs < 50000:
		return 5
	case numDirs < 100000:
		return 6
	case numDirs < 250000:
		return 7
	case numDirs < 500000:
		return 8
	case numDirs < 1000000:
		return 9
	default:
		return 10
	}
}

func calculateComplexity(fullScanTime int, numDirs uint64) uint {
	timeScore := calculateTimeScore(fullScanTime)
	dirScore := calculateDirScore(numDirs)
	complexity := timeScore
	if dirScore > timeScore {
		complexity = dirScore
	}
	return complexity
}

var fullScanAnchor = 3 // index of the schedule for a full scan

// markFilesChanged marks that files have changed in the currently active scanner
func (idx *Index) markFilesChanged() {
	activePath := idx.getActiveScannerPath()
	if activePath == "" {
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
	// Use unlocked version since we now hold the lock
	activePath := idx.getActiveScannerPathUnlocked()
	if activePath == "" {
		idx.mu.RUnlock()
		return
	}
	scanner, exists := idx.scanners[activePath]
	idx.mu.RUnlock()

	if exists {
		scanner.numDirs++
	}
}

// incrementScannerDirsUnlocked increments the directory counter for the active scanner
func (idx *Index) incrementScannerDirsUnlocked() {
	activePath := idx.getActiveScannerPathUnlocked()
	if activePath == "" {
		return
	}
	scanner, exists := idx.scanners[activePath]
	if exists {
		scanner.numDirs++
	}
}

// incrementScannerFiles increments the file counter for the active scanner
func (idx *Index) incrementScannerFiles() {
	idx.mu.RLock()
	// Use unlocked version since we now hold the lock
	activePath := idx.getActiveScannerPathUnlocked()
	if activePath == "" {
		idx.mu.RUnlock()
		return
	}
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
	// Only do expensive operations when ALL scanners are done
	if idx.getRunningScannerCount() == 0 {
		// All scanners completed - update root directory size and send event
		idx.updateRootDirectorySize()
		err := idx.db.ShrinkMemory()
		if err != nil {
			logger.Errorf("Failed to shrink memory: %v", err)
		}

		// Send update event to notify frontend that stats may have changed
		if err := idx.SendSourceUpdateEvent(); err != nil {
			logger.Errorf("Error sending source update event: %v", err)
		}

		// Clear scan session tracking when all scanners complete
		idx.mu.Lock()
		idx.scanSessionStartTime = 0
		idx.scanUpdatedPaths = make(map[string]bool) // Clear tracking map
		idx.mu.Unlock()
		return idx.SetStatus(READY)
	}
	// Scanners still running - skip expensive operations
	return nil
}

func (idx *Index) updateRootDirectorySize() {
	children, err := idx.db.GetDirectoryChildren(idx.Name, "/")
	if err != nil {
		logger.Errorf("Failed to get root directory children: %v", err)
		return
	}

	var totalSize int64
	for _, child := range children {
		totalSize += child.Size
	}

	// Check if root directory was updated by the scan itself
	idx.mu.RLock()
	wasUpdatedByScan := idx.scanUpdatedPaths["/"]
	scanSessionStartTime := idx.scanSessionStartTime
	idx.mu.RUnlock()

	if wasUpdatedByScan {
		// Root directory was updated by the scan - always update size (no timestamp check)
		if err := idx.db.UpdateDirectorySize(idx.Name, "/", totalSize); err != nil {
			logger.Errorf("Failed to update root directory size: %v", err)
		}
	} else {
		// Root directory existed before scan - use timestamp checking to avoid overwriting API updates
		_, err := idx.db.UpdateDirectorySizeIfStale(idx.Name, "/", totalSize, scanSessionStartTime)
		if err != nil {
			logger.Errorf("Failed to update root directory size: %v", err)
		}
	}
}

func (idx *Index) SendSourceUpdateEvent() error {
	if idx.mock {
		logger.Debug("Skipping source update event for mock index.")
		return nil
	}
	reducedIndex, err := GetIndexInfo(idx.Name, true)
	if err != nil {
		logger.Errorf("[%s] Error getting index info: %v", idx.Name, err)
		return err
	}
	sourceAsMap := map[string]ReducedIndex{
		idx.Name: reducedIndex,
	}
	message, err := json.Marshal(sourceAsMap)
	if err != nil {
		logger.Errorf("[%s] Error marshaling source update: %v", idx.Name, err)
		return err
	}
	quotedMessage := strconv.Quote(string(message))
	events.SendSourceUpdate(idx.Name, quotedMessage)
	return nil
}

// setupMultiScanner creates and starts the multi-scanner system
func (idx *Index) setupMultiScanner() {
	idx.mu.Lock()
	idx.scanners = make(map[string]*Scanner)
	idx.mu.Unlock()

	// Create and start root scanner
	rootScanner := idx.createRootScanner()
	idx.mu.Lock()
	idx.scanners["/"] = rootScanner
	idx.mu.Unlock()
	go rootScanner.start()

	// Discover existing top-level directories (root scanner will create scanners for new ones dynamically)
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
		idx:             idx,
		stopChan:        make(chan struct{}),
		currentSchedule: 0,
		fullScanCounter: 0,
		complexity:      0, // 0 = unknown until first full scan completes
	}
}

// createChildScanner creates a scanner for a specific child directory (recursive)
func (idx *Index) createChildScanner(dirPath string) *Scanner {
	return &Scanner{
		scanPath:        dirPath,
		idx:             idx,
		stopChan:        make(chan struct{}),
		currentSchedule: 0,
		fullScanCounter: 0,
		complexity:      0, // 0 = unknown until first full scan completes
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

	activePath := idx.getActiveScannerPathUnlocked()
	status["activeScanner"] = activePath
	status["isScanning"] = activePath != ""

	// Individual scanner stats
	scannerStats := make([]map[string]interface{}, 0, len(idx.scanners))
	for path, scanner := range idx.scanners {
		scannerInfo := map[string]interface{}{
			"path":            path,
			"lastScanned":     scanner.lastScanned.Format(time.RFC3339),
			"complexity":      scanner.complexity,
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
