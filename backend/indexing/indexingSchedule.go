package indexing

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/events"
)

// schedule in minutes
var scanSchedule = []time.Duration{
	5 * time.Minute, // 5 minute quick scan & 25 minutes for a full scan
	10 * time.Minute,
	20 * time.Minute,
	40 * time.Minute, // [3] element is 40 minutes, reset anchor for full scan
	1 * time.Hour,
	2 * time.Hour,
	3 * time.Hour,
	4 * time.Hour, // 4 hours for quick scan & 20 hours for a full scan
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
		if idx.Source.Config.IndexingInterval > 0 {
			sleepTime = time.Duration(idx.Source.Config.IndexingInterval) * time.Minute
		}
		// Log and sleep before indexing
		logger.Debug(fmt.Sprintf("Next scan in %v", sleepTime))
		time.Sleep(sleepTime)
		idx.PreScan()
		if fullScanCounter == 5 {
			idx.RunIndexing(origin, false) // Full scan
			fullScanCounter = 0
		} else {
			idx.RunIndexing(origin, true) // Quick scan
		}
		idx.PostScan()
		idx.UpdateSchedule()
	}
}

func (idx *Index) PreScan() {
	idx.scannerMu.Lock()
	idx.mu.Lock()
	idx.runningScannerCount++
	idx.mu.Unlock()
	SendSourceUpdateEvent()
}

func (idx *Index) PostScan() {
	idx.mu.Lock()
	idx.runningScannerCount--
	idx.mu.Unlock()
	idx.scannerMu.Unlock()
	SendSourceUpdateEvent()
}

func (idx *Index) UpdateSchedule() {
	// Adjust schedule based on file changes
	if idx.FilesChangedDuringIndexing {
		logger.Debug(fmt.Sprintf("Files changed during indexing [%v], adjusting schedule.", idx.Name))
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

func SendSourceUpdateEvent() {
	message, err := json.Marshal(GetIndexesInfo())
	if err != nil {
		logger.Error(fmt.Sprintf("Error marshalling source update message: %v", err))
		return
	}
	msg := events.EventMessage{
		EventType: "sourceUpdate",
		Message:   string(message),
	}
	events.BroadcastChan <- msg
	logger.Info("Broadcasted source update to all SSE clients.")
}

func (idx *Index) RunIndexing(origin string, quick bool) {
	prevNumDirs := idx.NumDirs
	prevNumFiles := idx.NumFiles
	if quick {
		logger.Debug(fmt.Sprintf("Starting quick scan for [%v]", idx.Source.Name))
	} else {
		logger.Debug(fmt.Sprintf("Starting full scan for [%v]", idx.Source.Name))
		idx.NumDirs = 0
		idx.NumFiles = 0
	}
	startTime := time.Now()
	idx.FilesChangedDuringIndexing = false
	// Perform the indexing operation
	err := idx.indexDirectory("/", quick, true)
	if err != nil {
		logger.Error(fmt.Sprintf("Error during indexing: %v", err))
	}
	firstRun := idx.LastIndexed == time.Time{}
	// Update the LastIndexed time
	idx.LastIndexed = time.Now()
	idx.IndexingTime = int(time.Since(startTime).Seconds())
	if !quick {
		// update smart indexing
		if idx.IndexingTime < 3 || idx.NumDirs < 10000 {
			idx.Assessment = "simple"
			idx.SmartModifier = 4 * time.Minute
		} else if idx.IndexingTime > 120 || idx.NumDirs > 500000 {
			idx.Assessment = "complex"
			modifier := idx.IndexingTime / 10 // seconds
			idx.SmartModifier = time.Duration(modifier) * time.Minute
		} else {
			idx.Assessment = "normal"
		}
		if firstRun {
			logger.Info(fmt.Sprintf("Index assessment         : [%v] complexity=%v directories=%v files=%v", idx.Source.Name, idx.Assessment, idx.NumDirs, idx.NumFiles))
		} else {
			logger.Debug(fmt.Sprintf("Index assessment         : [%v] complexity=%v directories=%v files=%v", idx.Source.Name, idx.Assessment, idx.NumDirs, idx.NumFiles))
		}
		if idx.NumDirs != prevNumDirs || idx.NumFiles != prevNumFiles {
			idx.FilesChangedDuringIndexing = true
		}
	}
	if firstRun {
		logger.Info(fmt.Sprintf("Time spent indexing [%v]: %v seconds", idx.Source.Name, idx.IndexingTime))
	} else {
		logger.Debug(fmt.Sprintf("Time spent indexing [%v]: %v seconds", idx.Source.Name, idx.IndexingTime))
	}
}

func (idx *Index) setupIndexingScanners() {
	go idx.newScanner("/")
}
