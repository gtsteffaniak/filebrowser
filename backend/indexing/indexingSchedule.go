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
	idx.wasIndexed = true
	idx.runningScannerCount--
	idx.mu.Unlock()
	if idx.runningScannerCount == 0 {
		return idx.SetStatus(READY)
	}
	return nil
}

// updateRootDirectorySize recalculates the "/" directory size from the database
// and updates the "/" entry. This should be called after any scanner completes.
func (idx *Index) updateRootDirectorySize() {
	// Get all direct children of root directory
	children, err := idx.db.GetDirectoryChildren(idx.Name, "/")
	if err != nil {
		logger.Errorf("Failed to get root directory children: %v", err)
		return
	}

	// Calculate total size: sum of all direct files + all child directory sizes
	var totalSize int64
	for _, child := range children {
		totalSize += child.Size
	}

	// Get current root directory entry
	rootDir, err := idx.db.GetItem(idx.Name, "/")
	if err != nil || rootDir == nil {
		logger.Errorf("Failed to get root directory entry: %v", err)
		return
	}

	// Update root directory size
	rootDir.Size = totalSize
	if err := idx.db.InsertItem(idx.Name, "/", rootDir); err != nil {
		logger.Errorf("Failed to update root directory size: %v", err)
		return
	}

	// Now aggregate stats and send update event
	idx.aggregateStatsFromScanners()
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
	// Quote the JSON string so it's sent as a string in the SSE message, not as an object
	// The sendEvent function expects the message to be properly quoted (like "\"connection established\"")
	quotedMessage := strconv.Quote(string(message))
	events.SendSourceUpdate(idx.Name, quotedMessage)
	return nil
}

// setupMultiScanner creates and starts the multi-scanner system
// Creates a root scanner (non-recursive) and child scanners for each top-level directory
func (idx *Index) setupMultiScanner() {
	logger.Infof("Setting up multi-scanner system for [%v]", idx.Name)

	idx.mu.Lock()
	idx.scanners = make(map[string]*Scanner)
	idx.initialScanStartTime = time.Now()
	idx.hasLoggedInitialScan = false
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

// aggregateStatsFromScanners aggregates stats from all scanners to Index-level stats
func (idx *Index) aggregateStatsFromScanners() {
	idx.mu.Lock()

	if len(idx.scanners) == 0 {
		idx.mu.Unlock()
		return
	}

	// Store previous stats and status to detect changes
	// Use stored previous totalSize for change detection
	prevNumDirs := idx.NumDirs
	prevNumFiles := idx.NumFiles
	prevDiskUsed := idx.previousTotalSize // Use stored previous value (0 on first call, which will trigger initial event)
	prevStatus := idx.Status

	// Aggregate stats from all scanners
	var totalDirs uint64 = 0
	var totalFiles uint64 = 0
	var totalQuickScanTime = 0
	var totalFullScanTime = 0
	var mostRecentScan time.Time
	allScannedAtLeastOnce := true

	for _, scanner := range idx.scanners {
		totalDirs += scanner.numDirs
		totalFiles += scanner.numFiles
		totalQuickScanTime += scanner.quickScanTime
		totalFullScanTime += scanner.fullScanTime
		if scanner.lastScanned.After(mostRecentScan) {
			mostRecentScan = scanner.lastScanned
		}
		if scanner.lastScanned.IsZero() {
			allScannedAtLeastOnce = false
		}
	}

	// Get total size directly from database (sum of all file sizes)
	// This is the source of truth for accurate size calculation
	dbTotalSize, err := idx.db.GetTotalSize(idx.Name)
	if err != nil {
		logger.Errorf("Failed to get total size from database: %v", err)
	} else {
		idx.totalSize = dbTotalSize
	}

	idx.NumDirs = totalDirs
	idx.NumFiles = totalFiles
	idx.QuickScanTime = totalQuickScanTime
	idx.FullScanTime = totalFullScanTime
	prevComplexity := idx.Complexity
	if allScannedAtLeastOnce {
		idx.Complexity = calculateComplexity(totalFullScanTime, totalDirs)
	} else {
		idx.Complexity = 0
	}
	complexityChanged := prevComplexity != idx.Complexity
	if !mostRecentScan.IsZero() {
		idx.LastIndexed = mostRecentScan
		idx.LastIndexedUnix = mostRecentScan.Unix()
		idx.wasIndexed = true
	}
	if allScannedAtLeastOnce && !idx.hasLoggedInitialScan {
		totalDuration := time.Since(idx.initialScanStartTime)
		truncatedToSecond := totalDuration.Truncate(time.Second)
		logger.Debugf("Time spent indexing [%v]: %v seconds", idx.Name, truncatedToSecond)
		idx.hasLoggedInitialScan = true
	}
	if allScannedAtLeastOnce && idx.activeScannerPath == "" {
		idx.Status = READY
	} else if idx.activeScannerPath != "" {
		idx.Status = INDEXING
	}
	newDiskUsed := idx.totalSize
	newStatus := idx.Status
	idx.mu.Unlock()

	// Update cache size when complexity changes or when all scans complete for the first time
	if complexityChanged && allScannedAtLeastOnce {
		updateIndexDBCacheSize()
	}

	statsChanged := prevNumDirs != totalDirs || prevNumFiles != totalFiles || prevDiskUsed != newDiskUsed
	statusChanged := prevStatus != newStatus
	if statsChanged || statusChanged {
		err := idx.SendSourceUpdateEvent()
		if err != nil {
			logger.Errorf("Error sending source update event: %v", err)
		}
		// Update previousTotalSize after sending event so next aggregation can detect changes
		idx.mu.Lock()
		idx.previousTotalSize = newDiskUsed
		idx.mu.Unlock()
	}
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
