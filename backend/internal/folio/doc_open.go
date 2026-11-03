package folio

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gen2brain/folio/doc"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

const folioTextCheckWindow = 4096 // html.readsAsText validation window

// maxTextPreviewLoad is the most plain-text we read for a first-page thumbnail.
const maxTextPreviewLoad = 32 << 10 // 32 KiB

// SupportsPreview reports whether folio should attempt this file (name + MIME).
// doc.Open sniffs content; extensions like .css are not in FolioConvertable but still render.
func SupportsPreview(name, mimetype string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if iteminfo.FolioConvertable[ext] {
		return true
	}
	if strings.HasPrefix(mimetype, "text/") {
		return true
	}
	return iteminfo.IsText(ext)
}

// useTextPreviewPrefix reports plain UTF-8 text previews (css, logs, .txt, …).
// These use doc.Load on a bounded prefix instead of doc.Open, which would read the whole file.
func useTextPreviewPrefix(name, mimetype string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if iteminfo.FolioConvertable[ext] {
		switch ext {
		case ".txt", ".md":
			return true
		default:
			return false
		}
	}
	if strings.HasPrefix(mimetype, "text/") {
		return ext != ".html" && ext != ".htm"
	}
	return iteminfo.IsText(ext)
}

func openDocument(path, name, mimetype string) (doc.Document, error) {
	if useTextPreviewPrefix(name, mimetype) {
		return openTextPreviewPrefix(path)
	}
	return doc.Open(path)
}

func openTextPreviewPrefix(path string) (doc.Document, error) {
	b, err := readTextPreviewPrefix(path, maxTextPreviewLoad)
	if err != nil {
		return nil, err
	}
	b = repairFolioTextValidationWindow(b)
	if len(b) == 0 {
		return nil, fmt.Errorf("text document is empty or not valid utf-8")
	}
	return doc.Load(b)
}

func readTextPreviewPrefix(path string, max int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := io.ReadAll(io.LimitReader(f, int64(max)))
	if err != nil {
		return nil, err
	}
	for len(b) > 0 && !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return b, nil
}

// repairFolioTextValidationWindow fixes folio/html readsAsText, which validates only
// the first 4096 bytes as UTF-8. A valid UTF-8 file can fail when a rune spans that boundary.
func repairFolioTextValidationWindow(b []byte) []byte {
	for len(b) > folioTextCheckWindow && !utf8.Valid(b[:folioTextCheckWindow]) {
		next := removeRuneSpanningBoundary(b, folioTextCheckWindow)
		if len(next) == len(b) {
			break
		}
		b = next
	}
	return b
}

func removeRuneSpanningBoundary(b []byte, boundary int) []byte {
	if boundary <= 0 || boundary > len(b) || utf8.Valid(b[:boundary]) {
		return b
	}
	start := boundary - 1
	for start > 0 && !utf8.RuneStart(b[start]) {
		start--
	}
	if !utf8.RuneStart(b[start]) {
		return b
	}
	_, size := utf8.DecodeRune(b[start:])
	if start+size <= boundary {
		return b
	}
	end := start + size
	if end > len(b) {
		end = len(b)
	}
	out := make([]byte, 0, len(b)-(end-start))
	out = append(out, b[:start]...)
	out = append(out, b[end:]...)
	return out
}
