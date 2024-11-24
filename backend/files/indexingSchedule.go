package files

import (
	"log"
	"time"

	"github.com/gtsteffaniak/filebrowser/settings"
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
	fullScanCounter := 0 // every 5th scan is full scan
	for {
		fullScanCounter++
		sleepTime := scanSchedule[si.currentSchedule] + si.SmartModifier
		if si.assessment == "simple" {
			sleepTime = scanSchedule[si.currentSchedule] - si.SmartModifier
		}
		if settings.Config.Server.IndexingInterval > 0 {
			sleepTime = time.Duration(settings.Config.Server.IndexingInterval) * time.Minute
		}
		log.Printf("Next scan in %v\n", sleepTime)
		time.Sleep(sleepTime)
		si.scannerMu.Lock()
		if fullScanCounter == 5 {
			si.RunIndexing(origin, false)
			fullScanCounter = 0
		} else {
			si.RunIndexing(origin, true)
		}
		si.scannerMu.Unlock()
		if si.FilesChangedDuringIndexing {
			// If files changed, adjust `i` to at least the minimum scan level (40 mins)
			if si.currentSchedule < fullScanAnchor {
				si.currentSchedule = fullScanAnchor
			} else {
				si.currentSchedule-- // Move closer to shorter intervals
			}
		} else {
			// If no files changed, increment `i` toward the longest interval
			if si.currentSchedule < len(scanSchedule)-1 {
				si.currentSchedule++
			}
		}
	}
}

func (si *Index) RunIndexing(origin string, quick bool) {
	if quick {
		log.Println("Starting quick scan")
	} else {
		log.Println("Starting full scan")
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
		if si.indexingTime < 2 || si.NumDirs < 1000 {
			si.assessment = "simple"
			si.SmartModifier = 4 * time.Minute
			log.Println("Index is small and efficiency, quick scan set to every minute")
		} else if si.indexingTime > 120 || si.NumDirs > 500000 {
			si.assessment = "complex"
			modifier := si.indexingTime / 10 // seconds
			si.SmartModifier = time.Duration(modifier) * time.Minute
			log.Println("Index is large and complex, quick scan set to every 10 minutes and complete scan happens less frequently")
		} else {
			si.assessment = "normal"
			log.Println("Index is normal, quick scan set to every 5 minutes.")
		}
		log.Printf("Index assessment         : complexity=%v directories=%v files=%v \n", si.assessment, si.NumDirs, si.NumFiles)
	}
	// Reset the indexing flag to indicate that indexing has finished
	si.inProgress = false
	log.Printf("Time Spent Indexing      : %v seconds\n", si.indexingTime)
}

func (si *Index) setupIndexingScanners() {
	go si.newScanner("/")
}
