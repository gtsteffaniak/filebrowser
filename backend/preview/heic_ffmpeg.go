package preview

import (
	"context"
	"fmt"
)

// convertHEICToJPEGWithFFmpeg converts a HEIC file to JPEG format using FFmpeg
// This function handles all FFmpeg-related logic and parameters
func (s *Service) convertHEICToJPEGWithFFmpeg(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Use the shared image service
	if s.imageService == nil {
		return nil, fmt.Errorf("image service not available")
	}

	// Acquire image service semaphore
	if err := s.imageService.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.imageService.Release()

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
	result, err := s.imageService.ConvertHEICToJPEG(ctx, filePath, width, height, quality)
	if err != nil {
		return nil, err
	}
	return result, nil
}
