//go:build !mupdf

package fulltext

import "time"

// documentText is unavailable without MuPDF; documents are indexed by name only.
func documentText(path string, timeout time.Duration) string {
	return ""
}
