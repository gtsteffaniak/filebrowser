//go:build mupdf

package fulltext

import (
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/go-fitz"
)

// fitzMu serializes MuPDF document access; go-fitz/MuPDF is not safe for
// concurrent use across documents.
var fitzMu sync.Mutex

// documentText extracts text from the first pages of a document.
// ponytail: first 50 pages only; raise capMaxPages if full-document search matters.
const maxDocumentPages = 50

func documentText(path string, timeout time.Duration) string {
	fitzMu.Lock()
	defer fitzMu.Unlock()

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
		if time.Since(start) > timeout || sb.Len() > MaxPlainTextBytes {
			break
		}
		text, err := doc.Text(i)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteByte('\n')
	}
	return strings.ToValidUTF8(sb.String(), "")
}
