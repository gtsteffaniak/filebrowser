package preview

import (
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
)

// convertHEICToJPEGWithFFmpeg converts a HEIC file to JPEG format using FFmpeg
// This function handles all FFmpeg-related logic and parameters
func (s *Service) convertHEICToJPEGWithFFmpeg(filePath string, previewSize string) ([]byte, error) {
	// Create FFmpeg image service
	imageService := ffmpeg.NewImageService(s.ffmpegPath, s.ffprobePath, s.debug, filepath.Join(settings.Config.Server.CacheDir, "heic", filepath.Base(filePath)))
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
	result, err := imageService.ConvertHEICToJPEG(filePath, width, height, quality)
	if err != nil {
		return nil, err
	}
	return result, nil
}
