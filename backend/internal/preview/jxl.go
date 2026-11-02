package preview

import (
	"context"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// generateJXLPreview decodes JPEG XL with pure Go and returns JPEG bytes for CreatePreview.
func (s *Service) generateJXLPreview(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize string) ([]byte, error) {
	imageBytes, err := s.convertJXLToJPEG(ctx, file.RealPath, previewSize)
	if err != nil {
		return nil, fmt.Errorf("failed to process JXL image file: %w", err)
	}
	return imageBytes, nil
}

func (s *Service) convertJXLToJPEG(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg service is not available")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var quality string
	switch previewSize {
	case "large":
		quality = "2"
	case "original":
		quality = "1"
	default:
		quality = "5"
	}

	return s.ffmpegService.ConvertJXLToJPEG(ctx, filePath, 0, 0, quality)
}
