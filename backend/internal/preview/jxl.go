package preview

import (
	"context"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/internal/gomedia"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// generateJXLPreview decodes JPEG XL via go-media (gen2brain/jxl) and returns JPEG bytes for CreatePreview.
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

	qualityStr := heicLegacyQualityString(previewSize)
	goQuality := ffmpeg.PreviewQualityFromLegacyQScale(qualityStr)

	shadowCtx := context.WithoutCancel(ctx)
	legacy := func() ([]byte, bool) {
		data, err := s.ffmpegService.ConvertImageToJPEG(shadowCtx, filePath, 0, 0, qualityStr)
		return data, err == nil && len(data) > 0
	}
	if data, ok := gomedia.JXLJPEG(filePath, goQuality, legacy); ok {
		return data, nil
	}

	if settings.Env.IsDevMode {
		logger.Debugf("go-media jxl jpeg unavailable for %s; using ffmpeg", filePath)
	}
	return s.ffmpegService.ConvertImageToJPEG(ctx, filePath, 0, 0, qualityStr)
}
