//go:build mupdf

package preview

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/gen2brain/go-fitz"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func docEnabled() bool {
	// This function checks if the PDF support is enabled.
	// In a real implementation, this might check a build tag or configuration.
	return true
}

func (s *Service) GenerateImageFromDoc(ctx context.Context, file iteminfo.ExtendedFileInfo, tempFilePath string, pageNumber int) ([]byte, error) {
	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Serialize access to the entire go-fitz operation block (required for CGO thread safety)
	s.docGenMutex.Lock()
	defer s.docGenMutex.Unlock()

	// 2. Lock the current goroutine to a single OS thread for CGo calls
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	docPath := file.RealPath
	// copy file to a temporary location if needed
	if strings.HasPrefix(file.Type, "text") && !strings.HasSuffix(file.RealPath, ".txt") {
		originalFile, err := os.Open(file.RealPath)
		if err != nil {
			return nil, fmt.Errorf("text snippet: failed to open original file '%s': %w", file.RealPath, err)
		}
		defer originalFile.Close() // Ensure original file is closed

		buffer := make([]byte, 1024) // Buffer for up to 1KB
		n, readErr := originalFile.Read(buffer)
		if readErr != nil && readErr != io.EOF { // io.EOF is not an error if some bytes were read
			return nil, fmt.Errorf("text snippet: failed to read from original file '%s': %w", file.RealPath, readErr)
		}

		if n == 0 {
			return nil, fmt.Errorf("text snippet: original file '%s' is empty or unreadable", file.RealPath)
		} else {
			tempFile, err := os.Create(tempFilePath)
			if err != nil {
				return nil, fmt.Errorf("text snippet: failed to create temporary file '%s': %w", tempFilePath, err)
			}
			defer os.Remove(tempFilePath) // Ensure cleanup on error
			// Write the read content (up to 1KB or EOF) to the temporary file
			if _, err := tempFile.Write(buffer[:n]); err != nil {
				tempFile.Close()        // Attempt to close
				os.Remove(tempFilePath) // Clean up on error
				return nil, fmt.Errorf("text snippet: failed to write to temporary file '%s': %w", tempFilePath, err)
			}

			// Close the temporary file so it can be reliably opened by path by other processes/functions
			if err := tempFile.Close(); err != nil {
				os.Remove(tempFilePath) // Clean up on error
				return nil, fmt.Errorf("text snippet: failed to close temporary file '%s': %w", tempFilePath, err)
			}

			docPath = tempFilePath // Update docPath to point to the new temporary text snippet file
		}
	}
	doc, err := fitz.New(docPath) // This calls the CGo version
	if err != nil {
		// The error message you received: "failed to open PDF: fitz: cannot open memory"
		return nil, fmt.Errorf("failed to open PDF from memory for file '%s': %w", docPath, err)
	}
	defer doc.Close()

	// Get the image from the doc page
	// Ensure pageNumber is valid (e.g., >= 0 for 0-indexed or >= 1 for 1-indexed, check go-fitz docs)
	// And pageNumber < doc.NumPage()
	numPages := doc.NumPage()
	if pageNumber < 0 || pageNumber >= numPages { // Assuming 0-indexed for Image()
		return nil, fmt.Errorf("invalid page number %d for PDF with %d pages ('%s')", pageNumber, numPages, docPath)
	}

	img, err := doc.Image(pageNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get image from page %d of '%s': %w", pageNumber, docPath, err)
	}

	// Create a new buffer to hold the image bytes
	var buf bytes.Buffer

	// Encode the image directly into the buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpeg.DefaultQuality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode image to jpeg for '%s': %w", docPath, err)
	}

	// Return the byte slice from the buffer
	return buf.Bytes(), nil
}
