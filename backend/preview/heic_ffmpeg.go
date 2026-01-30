package preview

import (
	"context"
	"fmt"
)

// convertHEICToJPEGWithFFmpeg converts a HEIC file to JPEG format using FFmpeg
// This function handles all FFmpeg-related logic and parameters
func (s *Service) convertHEICToJPEGWithFFmpeg(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	// Check if FFmpeg service is available
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg is not available")
	}

	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Acquire FFmpeg service semaphore
	if err := s.ffmpegService.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.ffmpegService.Release()

	// Determine target dimensions and quality based on preview size
	var width, height int
	var quality string
	switch previewSize {
	case "large":
		width, height = 640, 640
		quality = "2" // High quality for FFmpeg -q:v
	case "original":
		// For original size - no scaling, maximum quality
		width, height = 0, 0 // Signal to not apply scaling
		quality = "1"        // Maximum quality for original
	default:
		width, height = 256, 256
		quality = "5" // Medium quality
	}

	// Use tile-based conversion for correct full-resolution image reconstruction
	return s.ffmpegService.ConvertHEICToJPEG(ctx, filePath, width, height, quality)
}

// convertImageWithFFmpeg converts any image file (including problematic JPEGs) to resized JPEG using FFmpeg
// This is used as a fallback for JPEG files that Go's standard decoder can't handle (extended sequential, etc.)
func (s *Service) convertImageWithFFmpeg(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	// Check if FFmpeg service is available
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg is not available for JPEG fallback")
	}

	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Acquire FFmpeg service semaphore
	if err := s.ffmpegService.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.ffmpegService.Release()

	// Determine target dimensions and quality based on preview size
	var width, height int
	var quality string
	switch previewSize {
	case "large":
		width, height = 640, 640
		quality = "2" // High quality for FFmpeg -q:v
	case "original":
		// For original size - no scaling, maximum quality
		width, height = 0, 0 // Signal to not apply scaling
		quality = "1"        // Maximum quality for original
	default:
		width, height = 256, 256
		quality = "5" // Medium quality
	}

	// Use direct conversion which works for all image formats including problematic JPEGs
	return s.ffmpegService.ConvertImageToJPEG(ctx, filePath, width, height, quality)
}
