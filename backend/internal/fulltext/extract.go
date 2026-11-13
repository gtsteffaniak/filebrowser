// Package fulltext extracts searchable text from files for the FTS5 content index.
package fulltext

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

const (
	// MaxPlainTextBytes caps how much of a text file is indexed.
	MaxPlainTextBytes = 1 << 20
	// MaxDocumentBytes caps document size handled by the MuPDF text extractor.
	MaxDocumentBytes = 50 << 20
	// extractTimeout bounds one document extraction so a slow file cannot stall a scan.
	extractTimeout = 5 * time.Second
)

// textExtensions are indexed as raw text even when extension has no MIME mapping.
var textExtensions = map[string]bool{
	".txt": true, ".md": true, ".markdown": true, ".json": true, ".yaml": true, ".yml": true,
	".toml": true, ".ini": true, ".conf": true, ".csv": true, ".tsv": true, ".log": true,
	".go": true, ".py": true, ".js": true, ".ts": true, ".sh": true, ".rs": true, ".java": true,
	".c": true, ".h": true, ".cpp": true, ".hpp": true, ".css": true, ".html": true, ".xml": true,
	".sql": true, ".mk": true,
}

// Extract returns indexable text for the file: the file name (always searchable)
// followed by its content for supported types. Nil means the name alone is indexed.
// Called with symlink targets already resolved and containment-checked by the caller.
func Extract(realPath, name, mimeType string) *string {
	ext := strings.ToLower(filepath.Ext(name))

	// O_NONBLOCK keeps the open from waiting if the path was swapped for a
	// FIFO after the stat; the regular-file check below rejects it.
	f, err := os.OpenFile(realPath, os.O_RDONLY|oNonBlock, 0)
	if err != nil {
		return nil
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}

	var content string
	switch {
	case textExtensions[ext] || strings.HasPrefix(mimeType, "text/"):
		content = readPlainText(f)
	case iteminfo.IsDoc(ext) || iteminfo.MuPdfConvertable[ext]:
		if info.Size() == 0 || info.Size() > MaxDocumentBytes {
			return nameOnly(name)
		}
		content = documentText(realPath, extractTimeout)
	default:
		return nil
	}
	body := name
	if content != "" {
		body += "\n" + content
	}
	return &body
}

// nameOnly returns the name-only body for an eligible file that could not be read.
func nameOnly(name string) *string {
	return &name
}

// readPlainText reads at most MaxPlainTextBytes. Binary content (NUL in the head)
// yields "" so the file is indexed by name only.
func readPlainText(f *os.File) string {
	buf := make([]byte, MaxPlainTextBytes)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return ""
	}
	buf = buf[:n]
	if bytes.IndexByte(buf, 0) >= 0 {
		return ""
	}
	return strings.ToValidUTF8(string(buf), "")
}
