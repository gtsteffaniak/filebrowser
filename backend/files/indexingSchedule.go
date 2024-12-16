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

func (si *Index) newScanner(origin string) {
	fullScanAnchor := 3
	fullScanCounter := 0 // every 5th scan is a full scan
	for {
		// Determine sleep time with modifiers
		fullScanCounter++
		sleepTime := scanSchedule[si.currentSchedule] + si.SmartModifier
		if si.assessment == "simple" {
			sleepTime = scanSchedule[si.currentSchedule] - si.SmartModifier
		}
		if settings.Config.Server.IndexingInterval > 0 {
			sleepTime = time.Duration(settings.Config.Server.IndexingInterval) * time.Minute
		}

		// Log and sleep before indexing
		log.Printf("Next scan in %v\n", sleepTime)
		time.Sleep(sleepTime)

		si.scannerMu.Lock()
		if fullScanCounter == 5 {
			si.RunIndexing(origin, false) // Full scan
			fullScanCounter = 0
		} else {
			si.RunIndexing(origin, true) // Quick scan
		}
		si.scannerMu.Unlock()

		// Adjust schedule based on file changes
		if si.FilesChangedDuringIndexing {
			// Move to at least the full-scan anchor or reduce interval
			if si.currentSchedule > fullScanAnchor {
				si.currentSchedule = fullScanAnchor
			} else if si.currentSchedule > 0 {
				si.currentSchedule--
			}
		} else {
			// Increment toward the longest interval if no changes
			if si.currentSchedule < len(scanSchedule)-1 {
				si.currentSchedule++
			}
		}
		if si.assessment == "simple" && si.currentSchedule > 3 {
			si.currentSchedule = 3
		}
		// Ensure `currentSchedule` stays within bounds
		if si.currentSchedule < 0 {
			si.currentSchedule = 0
		} else if si.currentSchedule >= len(scanSchedule) {
			si.currentSchedule = len(scanSchedule) - 1
		}
	}
}

func (si *Index) RunIndexing(origin string, quick bool) {
	prevNumDirs := si.NumDirs
	prevNumFiles := si.NumFiles
	if quick {
		log.Println("Starting quick scan")
	} else {
		log.Println("Starting full scan")
		si.NumDirs = 0
		si.NumFiles = 0
	}
	startTime := time.Now()
	si.FilesChangedDuringIndexing = false
	// Perform the indexing operation
	err := si.indexDirectory("/", quick, true)
	if err != nil {
		log.Printf("Error during indexing: %v", err)
	}
	// Update the LastIndexed time
	si.LastIndexed = time.Now()
	si.indexingTime = int(time.Since(startTime).Seconds())
	if !quick {
		// update smart indexing
		if si.indexingTime < 3 || si.NumDirs < 10000 {
			si.assessment = "simple"
			si.SmartModifier = 4 * time.Minute
		} else if si.indexingTime > 120 || si.NumDirs > 500000 {
			si.assessment = "complex"
			modifier := si.indexingTime / 10 // seconds
			si.SmartModifier = time.Duration(modifier) * time.Minute
		} else {
			si.assessment = "normal"
		}
		log.Printf("Index assessment         : complexity=%v directories=%v files=%v \n", si.assessment, si.NumDirs, si.NumFiles)
		if si.NumDirs != prevNumDirs || si.NumFiles != prevNumFiles {
			si.FilesChangedDuringIndexing = true
		}
	}
	log.Printf("Time Spent Indexing      : %v seconds\n", si.indexingTime)
}

func (si *Index) setupIndexingScanners() {
	go si.newScanner("/")
}
