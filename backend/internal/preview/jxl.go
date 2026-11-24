package preview

import (
	"context"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/internal/gomedia"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// generateJXLPreview decodes JPEG XL via go-media and returns JPEG bytes for CreatePreview.
// There is no ffmpeg fallback: ffmpeg cannot decode JPEG XL.
func (s *Service) generateJXLPreview(ctx context.Context, file iteminfo.ExtendedFileInfo, previewSize string) ([]byte, error) {
	imageBytes, err := s.convertJXLToJPEG(ctx, file.RealPath, previewSize)
	if err != nil {
		return nil, fmt.Errorf("failed to process JXL image file: %w", err)
	}
	return imageBytes, nil
}

func (s *Service) convertJXLToJPEG(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	goQuality := ffmpeg.PreviewQualityFromLegacyQScale(heicLegacyQualityString(previewSize))

	// legacy is nil: no ffmpeg shadow comparison for JXL.
	if data, ok := gomedia.JXLJPEG(filePath, goQuality, nil); ok {
		return data, nil
	}
	return nil, fmt.Errorf("go-media could not decode JPEG XL file %s", filePath)
}
