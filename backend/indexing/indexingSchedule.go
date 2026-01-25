package indexing

import (
	"encoding/json"
	"strconv"
	"time"

	indexingdb "github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/events"
	"github.com/gtsteffaniak/go-logger/logger"
)

// schedule in minutes
var scanSchedule = map[int]time.Duration{
	0: 5 * time.Minute, // 5 minute scan
	1: 10 * time.Minute,
	2: 20 * time.Minute,
	3: 40 * time.Minute, // schedule index 3 for file changes
	4: 1 * time.Hour,
	5: 2 * time.Hour,
	6: 3 * time.Hour,
	7: 4 * time.Hour,
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

// calculateTimeScore returns a 1-10 score based on scan time
func calculateTimeScore(scanTime int) uint {
	if scanTime == 0 {
		return 1 // No data yet, assume simple
	}
	switch {
	case scanTime < 2:
		return 1
	case scanTime < 5:
		return 2
	case scanTime < 15:
		return 3
	case scanTime < 30:
		return 4
	case scanTime < 60:
		return 5
	case scanTime < 90:
		return 6
	case scanTime < 120:
		return 7
	case scanTime < 180:
		return 8
	case scanTime < 300:
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

func calculateComplexity(scanTime int, numDirs uint64) uint {
	timeScore := calculateTimeScore(scanTime)
	dirScore := calculateDirScore(numDirs)
	complexity := timeScore
	if dirScore > timeScore {
		complexity = dirScore
	}
	return complexity
}

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
		// All scanners completed
		err := idx.db.ShrinkMemory()
		if err != nil {
			logger.Errorf("Failed to shrink memory: %v", err)
		}

		// Persist index and scanner information (SSE event will be sent by Save())
		if err := idx.Save(); err != nil {
			logger.Errorf("Failed to save index persistence: %v", err)
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

func (idx *Index) SendSourceUpdateEvent() error {
	if idx.mock {
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
// isNewDb: if true, skip loading persisted complexity values (database is new or recreated)
func (idx *Index) setupMultiScanner(isNewDb bool) {
	// Load persisted index and scanner information
	if err := idx.Load(); err != nil {
		logger.Errorf("Failed to load persisted index data for [%v]: %v", idx.Name, err)
	}

	// Load existing folder sizes from database to avoid marking everything as dirty on startup
	existingSizes, err := idx.db.LoadFolderSizes(idx.Name)
	if err != nil {
		logger.Errorf("[INIT] Failed to load existing folder sizes for [%s]: %v", idx.Name, err)
	} else {
		idx.folderSizesMu.Lock()
		idx.folderSizes = existingSizes
		idx.folderSizesMu.Unlock()
	}

	idx.mu.Lock()
	idx.scanners = make(map[string]*Scanner)
	idx.mu.Unlock()

	// Load persisted scanner info if available
	var persistedScanners map[string]*indexingdb.PersistedScannerInfo
	if indexingStorage != nil && !isNewDb {
		info, err := indexingStorage.GetByPath(idx.Path)
		if err == nil && info != nil {
			persistedScanners = info.Scanners
		}
	}

	// Create and start root scanner
	rootScanner := idx.createScanner("/")
	if persistedScanners != nil {
		if rootInfo, ok := persistedScanners["/"]; ok {
			rootScanner.complexity = rootInfo.Complexity
			rootScanner.currentSchedule = rootInfo.CurrentSchedule
			rootScanner.quickScanTime = rootInfo.QuickScanTime
			rootScanner.fullScanTime = rootInfo.FullScanTime
			rootScanner.numDirs = rootInfo.NumDirs
			rootScanner.numFiles = rootInfo.NumFiles
			rootScanner.lastScanned = rootInfo.LastScanned
		}
	}
	idx.mu.Lock()
	idx.scanners["/"] = rootScanner
	idx.mu.Unlock()
	go rootScanner.start()

	// Discover existing top-level directories (root scanner will create scanners for new ones dynamically)
	topLevelDirs := rootScanner.getTopLevelDirs()

	// Create child scanner for each top-level directory
	for _, dirPath := range topLevelDirs {
		childScanner := idx.createScanner(dirPath)

		// Restore persisted stats for child scanner if available (and DB is not new)
		if persistedScanners != nil {
			if childInfo, ok := persistedScanners[dirPath]; ok {
				childScanner.complexity = childInfo.Complexity
				childScanner.currentSchedule = childInfo.CurrentSchedule
				childScanner.quickScanTime = childInfo.QuickScanTime
				childScanner.fullScanTime = childInfo.FullScanTime
				childScanner.numDirs = childInfo.NumDirs
				childScanner.numFiles = childInfo.NumFiles
				childScanner.lastScanned = childInfo.LastScanned
			}
		}

		idx.mu.Lock()
		idx.scanners[dirPath] = childScanner
		idx.mu.Unlock()

		go childScanner.start()
	}

	logger.Debugf("Created %d scanners for [%v] (1 root + %d children)", len(topLevelDirs)+1, idx.Name, len(topLevelDirs))
}

// createChildScanner creates a scanner for a specific child directory (recursive)
func (idx *Index) createScanner(dirPath string) *Scanner {
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
func (idx *Index) setupIndexingScanners(isNewDb bool) {
	// Use new multi-scanner system
	idx.setupMultiScanner(isNewDb)
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
