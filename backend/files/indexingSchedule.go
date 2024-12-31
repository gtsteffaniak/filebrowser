package files

import (
	"log"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/settings"
)

// schedule in minutes
var scanSchedule = []time.Duration{
	5 * time.Minute, // 5 minute quick scan & 25 minutes for a full scan
	10 * time.Minute,
	20 * time.Minute, // [3] element is 20 minutes, reset anchor for full scan
	40 * time.Minute,
	1 * time.Hour,
	2 * time.Hour,
	3 * time.Hour,
	4 * time.Hour, // 4 hours for quick scan & 20 hours for a full scan
}

func (idx *Index) newScanner(origin string) {
	fullScanAnchor := 3
	fullScanCounter := 0 // every 5th scan is a full scan
	for {
		// Determine sleep time with modifiers
		fullScanCounter++
		sleepTime := scanSchedule[idx.currentSchedule] + idx.SmartModifier
		if idx.assessment == "simple" {
			sleepTime = scanSchedule[idx.currentSchedule] - idx.SmartModifier
		}
		if settings.Config.Server.IndexingInterval > 0 {
			sleepTime = time.Duration(settings.Config.Server.IndexingInterval) * time.Minute
		}

		// Log and sleep before indexing
		log.Printf("Next scan in %v\n", sleepTime)
		time.Sleep(sleepTime)

		idx.scannerMu.Lock()
		if fullScanCounter == 5 {
			idx.RunIndexing(origin, false) // Full scan
			fullScanCounter = 0
		} else {
			idx.RunIndexing(origin, true) // Quick scan
		}
		idx.scannerMu.Unlock()

		// Adjust schedule based on file changes
		if idx.FilesChangedDuringIndexing {
			// Move to at least the full-scan anchor or reduce interval
			if idx.currentSchedule > fullScanAnchor {
				idx.currentSchedule = fullScanAnchor
			} else if idx.currentSchedule > 0 {
				idx.currentSchedule--
			}
		} else {
			// Increment toward the longest interval if no changes
			if idx.currentSchedule < len(scanSchedule)-1 {
				idx.currentSchedule++
			}
		}
		if idx.assessment == "simple" && idx.currentSchedule > 3 {
			idx.currentSchedule = 3
		}
		// Ensure `currentSchedule` stays within bounds
		if idx.currentSchedule < 0 {
			idx.currentSchedule = 0
		} else if idx.currentSchedule >= len(scanSchedule) {
			idx.currentSchedule = len(scanSchedule) - 1
		}
	}
}

func (idx *Index) RunIndexing(origin string, quick bool) {
	prevNumDirs := idx.NumDirs
	prevNumFiles := idx.NumFiles
	if quick {
		log.Println("Starting quick scan")
	} else {
		log.Println("Starting full scan")
		idx.NumDirs = 0
		idx.NumFiles = 0
	}
	startTime := time.Now()
	idx.FilesChangedDuringIndexing = false
	// Perform the indexing operation
	err := idx.indexDirectory("/", quick, true)
	if err != nil {
		log.Printf("Error during indexing: %v", err)
	}
	// Update the LastIndexed time
	idx.LastIndexed = time.Now()
	idx.indexingTime = int(time.Since(startTime).Seconds())
	if !quick {
		// update smart indexing
		if idx.indexingTime < 3 || idx.NumDirs < 10000 {
			idx.assessment = "simple"
			idx.SmartModifier = 4 * time.Minute
		} else if idx.indexingTime > 120 || idx.NumDirs > 500000 {
			idx.assessment = "complex"
			modifier := idx.indexingTime / 10 // seconds
			idx.SmartModifier = time.Duration(modifier) * time.Minute
		} else {
			idx.assessment = "normal"
		}
		log.Printf("Index assessment         : complexity=%v directories=%v files=%v \n", idx.assessment, idx.NumDirs, idx.NumFiles)
		if idx.NumDirs != prevNumDirs || idx.NumFiles != prevNumFiles {
			idx.FilesChangedDuringIndexing = true
		}
	}
	log.Printf("Time Spent Indexing      : %v seconds\n", idx.indexingTime)
}

func (idx *Index) setupIndexingScanners() {
	go idx.newScanner("/")
}
