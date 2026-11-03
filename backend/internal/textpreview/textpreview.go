// Package textpreview reads a size-capped prefix of a text-like file for the text preview API.
//
// It does not use the image preview pipeline and does not sanitize output. The frontend
// is responsible for sanitizing everything it renders from this package.
package textpreview

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// MaxBytes is the largest amount of a file that is read for a preview.
const MaxBytes = 16 * 1024

// ErrNotText is returned for files that look binary (contain NUL bytes).
var ErrNotText = errors.New("file does not look like text")

// Result is the JSON payload returned by the text preview endpoint.
type Result struct {
	// Content is plain text, or an HTML fragment when Format is "html".
	Content string `json:"content"`
	// Kind is the renderer the frontend should use: plain, csv, markdown, html, css, code, or rtf.
	Kind string `json:"kind"`
	// Format is "text" or "html". RTF is converted to HTML; everything else is text.
	Format string `json:"format"`
	// Truncated is true when the file is larger than the returned content.
	Truncated bool `json:"truncated"`
	// Size is the full size of the file in bytes.
	Size int64 `json:"size"`
}

// Read returns up to MaxBytes of the file at realPath, prepared for the given renderer kind.
func Read(realPath, kind string) (Result, error) {
	f, err := os.Open(realPath)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return Result{}, err
	}

	buf := make([]byte, MaxBytes)
	n, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return Result{}, err
	}
	buf = buf[:n]

	if bytes.IndexByte(buf, 0) >= 0 {
		return Result{}, ErrNotText
	}

	truncated := info.Size() > int64(n)
	if truncated {
		buf = trimIncompleteRune(buf)
		buf = trimToLastLine(buf)
	}

	// Legacy encodings are not valid UTF-8. Replace the bad bytes so the preview still renders.
	text := strings.ToValidUTF8(string(buf), "\uFFFD")

	result := Result{Kind: kind, Format: "text", Truncated: truncated, Size: info.Size()}
	if kind == "rtf" {
		result.Content = RTFToHTML(text)
		result.Format = "html"
	} else {
		result.Content = text
	}
	return result, nil
}

// trimIncompleteRune removes a trailing partial UTF-8 sequence left by the read limit.
func trimIncompleteRune(b []byte) []byte {
	for i := 1; i <= utf8.UTFMax && i <= len(b); i++ {
		c := b[len(b)-i]
		if c&0xC0 == 0x80 {
			// Continuation byte: keep looking for the lead byte.
			continue
		}
		need := 1
		switch {
		case c&0xE0 == 0xC0:
			need = 2
		case c&0xF0 == 0xE0:
			need = 3
		case c&0xF8 == 0xF0:
			need = 4
		}
		if need > i {
			return b[:len(b)-i]
		}
		return b
	}
	return b
}

// trimToLastLine cuts at the last newline when that keeps at least half of the buffer.
// Files with very long lines keep their full prefix so the preview is not empty.
func trimToLastLine(b []byte) []byte {
	idx := bytes.LastIndexByte(b, '\n')
	if idx > len(b)/2 {
		return b[:idx]
	}
	return b
}
