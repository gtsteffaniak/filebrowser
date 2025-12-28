package indexing

import (
	"time"
)

// IndexInfo stores persistent information about an index and its scanners
type IndexInfo struct {
	// Index identifier - uses the real filesystem path (index.Path) as the key
	Path string `json:"path" storm:"id,index"`

	// Source name (index.Name) for reference
	Source string `json:"source" storm:"index"`

	// Index-level stats
	Complexity uint   `json:"complexity"`
	NumDirs    uint64 `json:"numDirs"`
	NumFiles   uint64 `json:"numFiles"`

	// Scanner information - map of scanner path to scanner stats
	Scanners map[string]*ScannerInfo `json:"scanners"`
}

// ScannerInfo stores persistent information about a scanner
type ScannerInfo struct {
	Path            string    `json:"path"`
	Complexity      uint      `json:"complexity"`
	CurrentSchedule int       `json:"currentSchedule"`
	QuickScanTime   int       `json:"quickScanTime"`
	FullScanTime    int       `json:"fullScanTime"`
	NumDirs         uint64    `json:"numDirs"`
	NumFiles        uint64    `json:"numFiles"`
	LastScanned     time.Time `json:"lastScanned"`
}

