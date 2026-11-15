package utils

import (
	"io"
	"net/http"
	"os"
	"unicode/utf8"
)

// IsTextFile checks if a file is viewable/editable as text (like with cat or a text editor).
// It returns true if the file appears to be text, false if it's likely binary.
// This function uses simple heuristics:
// - Valid UTF-8 encoding
// - Not full of null bytes (which indicate binary data)
func IsTextFile(realPath string) (bool, error) {
	const sampleSize = 8192       // Read first 8KB to check
	const maxNullByteRatio = 0.05 // Reject if more than 5% null bytes
	// Read only the first sampleSize bytes. The previous implementation
	// used os.ReadFile, which loaded the entire file into memory and
	// only then sliced off the first 8KB — so checking a multi-GB file
	// (e.g. on a file-watcher event) read the whole file for no reason.
	f, err := os.Open(realPath)
	if err != nil {
		return false, err
	}
	defer f.Close()
	buf := make([]byte, sampleSize)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	sample := buf[:n]
	// Empty files are considered text
	if len(sample) == 0 {
		return true, nil
	}
	// Check 1: Reject PDFs. A PDF with a long ASCII text layer looks like text
	// for far more than sampleSize bytes.
	if http.DetectContentType(sample) == "application/pdf" {
		return false, nil
	}

	// Check 2: Count null bytes - binary files typically have many nulls
	nullCount := 0
	for _, b := range sample {
		if b == 0x00 {
			nullCount++
		}
	}

	if nullCount > 0 {
		nullRatio := float64(nullCount) / float64(len(sample))
		if nullRatio > maxNullByteRatio {
			return false, nil
		}
	}

	// Check 3: Validate UTF-8 encoding
	// Trim sample to last complete UTF-8 rune to avoid false negatives
	trimmedSample := sample
	for len(trimmedSample) > 0 {
		lastRune, size := utf8.DecodeLastRune(trimmedSample)
		if lastRune != utf8.RuneError {
			break
		}
		if size == 1 {
			trimmedSample = trimmedSample[:len(trimmedSample)-1]
		} else {
			break
		}
	}

	if len(trimmedSample) > 0 && !utf8.Valid(trimmedSample) {
		return false, nil
	}

	return true, nil
}
