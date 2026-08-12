//go:build mupdf

package preview

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
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

	// Serialize access to the entire MuPDF operation block (required for CGO thread safety)
	s.docGenMutex.Lock()
	defer s.docGenMutex.Unlock()

	// Lock the current goroutine to a single OS thread for CGo calls
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Create a 2-second timeout for document generation after acquiring locks
	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

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

	// Check timeout before opening document
	if timeoutCtx.Err() != nil {
		return nil, fmt.Errorf("document preview generation timed out after 2 seconds for '%s'", file.Name)
	}

	imageBytes, err := renderDocPageJPEG(docPath, pageNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to create image for document '%s': %w", docPath, err)
	}

	// Check timeout after rendering image
	if timeoutCtx.Err() != nil {
		return nil, fmt.Errorf("document preview generation timed out after 2 seconds for '%s'", file.Name)
	}

	return imageBytes, nil
}
