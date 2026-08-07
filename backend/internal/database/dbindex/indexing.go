package dbindex

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

	// Disk usage (cached from periodic GetIndexInfo / partition probes; aligns with ReducedIndex Stats)
	UsedAsIndexed uint64 `json:"used"`
	UsedDisk      uint64 `json:"usedAlt"`
	DiskTotal     uint64 `json:"total"`

	// Scanner information - map of scanner path to scanner stats
	Scanners map[string]*PersistedScannerInfo `json:"scanners"`
}

// PersistedScannerInfo stores persistent information about a scanner
type PersistedScannerInfo struct {
	Path            string    `json:"path"`
	Complexity      uint      `json:"complexity"`
	CurrentSchedule int       `json:"currentSchedule"`
	NumDirs         uint64    `json:"numDirs"`
	NumFiles        uint64    `json:"numFiles"`
	QuickScanTime   int       `json:"quickScanTime"`
	FullScanTime    int       `json:"fullScanTime"`
	LastScanned     time.Time `json:"lastScanned"`
	FullScanCounter int       `json:"fullScanCounter"` // 0-4: position in 1 full + 4 quick cycle before next executeScan
}
