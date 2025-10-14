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

func (idx *Index) PreScan() error {
	return idx.SetStatus(INDEXING)
}

func (idx *Index) PostScan() error {
	idx.mu.Lock()
	idx.garbageCollection()
	idx.hasIndex = true
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
	config := &actionConfig{
		Quick:     quick,
		Recursive: true,
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

func (idx *Index) setupIndexingScanners() {
	go idx.newScanner("/")
}
