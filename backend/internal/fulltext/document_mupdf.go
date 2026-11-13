//go:build mupdf

package fulltext

import (
	"strings"
	"time"

	"github.com/gen2brain/go-fitz"
)

// documentText extracts text from the first pages of a document.
// ponytail: first 50 pages only; raise maxDocumentPages if full-document search matters.
const maxDocumentPages = 50

// documentText extracts up to maxDocumentPages of text, within timeout and the
// MaxPlainTextBytes budget.
//
// ponytail: MuPDF calls cannot be interrupted, so the timeout is checked between
// pages, not enforced mid-call; a hung page blocks extraction (and with it the
// scan) until MuPDF returns. Isolating that needs a worker process.
func documentText(path string, timeout time.Duration) string {
	doc, err := fitz.New(path)
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
		// Truncate at a rune boundary so the body never exceeds the cap.
		remaining := MaxPlainTextBytes - sb.Len()
		if len(text) > remaining {
			text = text[:remaining]
			for len(text) > 0 && !utf8Start(text[len(text)-1]) {
				text = text[:len(text)-1]
			}
		}
		sb.WriteString(text)
		sb.WriteByte('\n')
	}
	return strings.ToValidUTF8(sb.String(), "")
}

// utf8Start reports whether b can start a UTF-8 sequence (not a continuation byte).
func utf8Start(b byte) bool { return b&0xC0 != 0x80 }
