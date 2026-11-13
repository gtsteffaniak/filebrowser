//go:build mupdf

package fulltext

import (
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gen2brain/go-fitz"
)

// maxDocumentPages caps how many pages are extracted.
// ponytail: first 50 pages only; raise it if full-document search matters.
const maxDocumentPages = 50

// documentText extracts up to maxDocumentPages of text from the already-open,
// verified file, within timeout and the MaxPlainTextBytes budget.
//
// ponytail: MuPDF calls cannot be interrupted, so timeout is checked between
// pages, not enforced mid-call; a hung page blocks extraction (and with it the
// scan) until MuPDF returns. Isolating that needs a worker process.
func documentText(f *os.File, timeout time.Duration) string {
	doc, err := fitz.NewFromReader(f)
	if err != nil {
		return ""
	}
	defer doc.Close()

	var sb strings.Builder
	start := time.Now()
	pages := doc.NumPage()
	if pages > maxDocumentPages {
		pages = maxDocumentPages
	}
	for i := 0; i < pages; i++ {
		if time.Since(start) > timeout || sb.Len() >= MaxPlainTextBytes {
			break
		}
		text, err := doc.Text(i)
		if err != nil {
			continue
		}
		sb.WriteString(truncateRunes(text, MaxPlainTextBytes-sb.Len()))
		sb.WriteByte('\n')
	}
	return strings.ToValidUTF8(sb.String(), "")
}

// truncateRunes returns at most maxBytes of s, ending on a rune boundary so a
// complete multibyte rune that fits is never dropped.
func truncateRunes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
