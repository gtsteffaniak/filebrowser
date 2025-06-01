//go:build pdf
// +build pdf

package preview

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"

	"github.com/gen2brain/go-fitz"
)

func pdfEnabled() bool {
	// This function checks if the PDF support is enabled.
	// In a real implementation, this might check a build tag or configuration.
	return true
}

func (s *Service) GenerateImageFromPDF(pdfPath string, pageNumber int) ([]byte, error) {
	if err := s.acquire(context.Background()); err != nil {
		return nil, err
	}
	defer s.release()
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	// Get the image from the PDF page
	img, err := doc.Image(pageNumber) // Assuming page numbers are 0-indexed as per go-fitz common usage
	if err != nil {
		return nil, fmt.Errorf("failed to get image from page %d: %w", pageNumber, err)
	}

	// Create a new buffer to hold the image bytes
	var buf bytes.Buffer

	// Encode the image directly into the buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpeg.DefaultQuality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode image to jpeg: %w", err)
	}

	// Return the byte slice from the buffer
	return buf.Bytes(), nil
}
