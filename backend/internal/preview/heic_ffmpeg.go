package preview

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// convertHEICToJPEGWithFFmpeg converts HEIC to JPEG using libheif/heif-convert.
//
// FFmpeg has problems with some iPhone HEIC tile-grid images.
// heif-convert from libheif handles these files correctly.
func (s *Service) convertHEICToJPEGWithFFmpeg(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Create a temporary directory for the converted JPEG.
	tempDir, err := os.MkdirTemp("", "filebrowser-heic-")
	if err != nil {
		return nil, fmt.Errorf("create HEIC temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	outputPath := filepath.Join(tempDir, "preview.jpg")

	// Convert HEIC -> JPEG using libheif.
	// heif-convert correctly handles iPhone HEIC tile grids.
	cmd := exec.CommandContext(
		ctx,
		"/usr/bin/heif-convert",
		filePath,
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf(
			"heif-convert failed: %w: %s",
			err,
			string(output),
		)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Read the converted JPEG into memory.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read converted HEIC JPEG: %w", err)
	}

	return data, nil
}

// convertImageWithFFmpeg converts any image file (including problematic JPEGs)
// to resized JPEG using FFmpeg.
//
// This is used as a fallback for JPEG files that Go's standard decoder can't
// handle (extended sequential, etc.).
func (s *Service) convertImageWithFFmpeg(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg is not available for JPEG fallback")
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if err := s.ffmpegService.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.ffmpegService.Release()

	var width, height int
	var quality string

	switch previewSize {
	case "large":
		width, height = 640, 640
		quality = "2"
	case "original":
		width, height = 0, 0
		quality = "1"
	default:
		width, height = 256, 256
		quality = "5"
	}

	return s.ffmpegService.ConvertImageToJPEG(
		ctx,
		filePath,
		width,
		height,
		quality,
	)
}
