//go:build !mupdf

package fulltext

import (
	"os"
	"time"
)

// documentText is unavailable without MuPDF; documents are indexed by name only.
func documentText(f *os.File, timeout time.Duration) string {
	return ""
}
