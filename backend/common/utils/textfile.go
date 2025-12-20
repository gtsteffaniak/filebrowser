package utils

import (
	"io"
	"os"
	"unicode"
	"unicode/utf8"
)

// IsTextFile checks if a file is likely a text file by examining its content.
// It returns true if the file appears to be a valid text file, false otherwise.
// This function performs various heuristics to detect text files:
// - Valid UTF-8 encoding
// - Limited null bytes
// - Limited non-printable characters
func IsTextFile(realPath string) (bool, error) {
	const headerSize = 4096
	// Thresholds for detecting binary-like content (these can be tuned)
	const maxNullBytesInHeaderAbs = 10    // Max absolute null bytes in header
	const maxNullByteRatioInHeader = 0.1  // Max 10% null bytes in header
	const maxNullByteRatioInFile = 0.05   // Max 5% null bytes in the entire file
	const maxNonPrintableRuneRatio = 0.05 // Max 5% non-printable runes in the entire file

	// Open file
	f, err := os.Open(realPath)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// Read header
	headerBytes := make([]byte, headerSize)
	n, err := f.Read(headerBytes)
	if err != nil && err != io.EOF {
		return false, err
	}
	actualHeader := headerBytes[:n]

	// --- Start of heuristic checks ---

	if n > 0 {
		// Trim header to last complete UTF-8 rune to avoid false negatives
		// when the header read cuts off in the middle of a multi-byte sequence.
		// We decode runes from the end until we find a valid one, trimming
		// any incomplete sequences at the end.
		trimmedHeader := actualHeader
		for len(trimmedHeader) > 0 {
			lastRune, size := utf8.DecodeLastRune(trimmedHeader)
			if lastRune != utf8.RuneError {
				// Found a valid complete rune
				break
			}
			// RuneError occurred - this could be an incomplete sequence or invalid byte
			// Trim the last byte and try again
			if size == 1 && len(trimmedHeader) > 0 {
				trimmedHeader = trimmedHeader[:len(trimmedHeader)-1]
			} else {
				// Shouldn't happen, but break to avoid infinite loop
				break
			}
		}

		// 1. Basic Check: Is the header valid UTF-8?
		// If not, it's unlikely an editable UTF-8 text file.
		// Use trimmed header to avoid false negatives from truncated sequences
		if len(trimmedHeader) > 0 && !utf8.Valid(trimmedHeader) {
			return false, nil // Not an error, just not the text file we want
		}

		// 2. Check for excessive null bytes in the header
		nullCountInHeader := 0
		for _, b := range actualHeader {
			if b == 0x00 {
				nullCountInHeader++
			}
		}
		// Reject if too many nulls absolutely or relatively in the header
		if nullCountInHeader > 0 { // Only perform check if there are any nulls
			if nullCountInHeader > maxNullBytesInHeaderAbs ||
				(float64(nullCountInHeader)/float64(n) > maxNullByteRatioInHeader) {
				return false, nil // Too many nulls in header
			}
		}

		// 3. Check for other non-text ASCII control characters in the header
		// (C0 controls excluding \t, \n, \r)
		for _, b := range actualHeader {
			if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
				return false, nil // Found problematic control character
			}
		}
	}

	// Now read the full file (original logic)
	content, err := os.ReadFile(realPath)
	if err != nil {
		return false, err
	}
	// Handle empty file (empty files are considered text)
	if len(content) == 0 {
		return true, nil
	}

	stringContent := string(content)

	// 4. Final UTF-8 validation for the entire file
	// (This is crucial as the header might be fine, but the rest of the file isn't)
	if !utf8.ValidString(stringContent) {
		return false, nil
	}

	// 5. Check for excessive null bytes in the entire file content
	if len(content) > 0 { // Check only for non-empty files
		totalNullCount := 0
		for _, b := range content {
			if b == 0x00 {
				totalNullCount++
			}
		}
		if float64(totalNullCount)/float64(len(content)) > maxNullByteRatioInFile {
			return false, nil // Too many nulls in the entire file
		}
	}

	// 6. Check for excessive non-printable runes in the entire file content
	// (Excluding tab, newline, carriage return, which are common in text files)
	if len(stringContent) > 0 { // Check only for non-empty strings
		nonPrintableRuneCount := 0
		totalRuneCount := 0
		for _, r := range stringContent {
			totalRuneCount++
			// unicode.IsPrint includes letters, numbers, punctuation, symbols, and spaces.
			// It excludes control characters. We explicitly allow \t, \n, \r.
			if !unicode.IsPrint(r) && r != '\t' && r != '\n' && r != '\r' {
				nonPrintableRuneCount++
			}
		}

		if totalRuneCount > 0 { // Avoid division by zero
			if float64(nonPrintableRuneCount)/float64(totalRuneCount) > maxNonPrintableRuneRatio {
				return false, nil // Too many non-printable runes
			}
		}
	}

	// The file has passed all checks and is considered editable text.
	return true, nil
}
