package preview

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

// convertHEICToJPEGWithFFmpeg converts a HEIC file to JPEG format using FFmpeg
// This function handles all FFmpeg-related logic and parameters
func (s *Service) convertHEICToJPEGWithFFmpeg(filePath string, previewSize string) ([]byte, error) {
	logger.Infof("🎯 HEIC FFMPEG: Starting conversion for %s (size: %s)", filepath.Base(filePath), previewSize)

	if s.ffmpegPath == "" {
		logger.Errorf("❌ HEIC FFMPEG: FFmpeg path is empty")
		return nil, fmt.Errorf("FFmpeg is not available for HEIC conversion")
	}

	logger.Infof("🔧 HEIC FFMPEG: Using FFmpeg at: %s", s.ffmpegPath)

	if err := s.acquire(context.Background()); err != nil {
		logger.Errorf("❌ HEIC FFMPEG: Failed to acquire semaphore: %v", err)
		return nil, err
	}
	defer s.release()

	// Create FFmpeg image service
	imageService := ffmpeg.NewImageService(s.ffmpegPath, s.ffprobePath, s.debug)

	// Determine target dimensions and quality based on preview size
	var width, height int
	var quality string

	switch previewSize {
	case "large":
		width, height = 640, 640
		quality = "2" // High quality for FFmpeg -q:v
		logger.Infof("📐 HEIC FFMPEG: Using LARGE size - target: %dx%d, quality: %s", width, height, quality)
	case "small":
		width, height = 256, 256
		quality = "5" // Medium quality
		logger.Infof("📐 HEIC FFMPEG: Using SMALL size - target: %dx%d, quality: %s", width, height, quality)
	case "original":
		// For original size - no scaling, maximum quality
		logger.Infof("📏 HEIC FFMPEG: Using ORIGINAL size - no scaling, maximum quality")
		width, height = 0, 0 // Signal to not apply scaling
		quality = "1"        // Maximum quality for original
		logger.Infof("📐 HEIC FFMPEG: Using ORIGINAL mode - no scaling, quality: %s", quality)
	default:
		logger.Errorf("❌ HEIC FFMPEG: Unsupported preview size: %s", previewSize)
		return nil, ErrUnsupportedFormat
	}

	logger.Infof("🚀 HEIC FFMPEG: Starting FFmpeg conversion with dimensions %dx%d, quality %s", width, height, quality)
	result, err := imageService.ConvertHEICToJPEG(filePath, width, height, quality)
	if err != nil {
		logger.Errorf("❌ HEIC FFMPEG: Conversion failed: %v", err)
		return nil, err
	}

	logger.Infof("✅ HEIC FFMPEG: Conversion successful, output size: %d bytes", len(result))
	return result, nil
}
