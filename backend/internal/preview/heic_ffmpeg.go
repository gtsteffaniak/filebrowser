package preview

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/internal/gomedia"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

func heicLegacyQualityString(previewSize string) string {
	switch previewSize {
	case "large":
		return "2"
	case "original":
		return "1"
	default:
		return "5"
	}
}

// heicDecodeBounds returns the box go-media decodes HEIC into for previewSize.
// Only fit-mode previews are bounded: the decode then already fits the box, and
// CreatePreview returns it unchanged. small crops to fill, which needs pixels
// beyond its box, so it and original decode at full resolution (0, 0).
func heicDecodeBounds(previewSize string) (int, int) {
	opts, err := getPreviewOptions(previewSize)
	if err != nil || opts.ResizeMode != ResizeModeFit {
		return 0, 0
	}
	return opts.Width, opts.Height
}

// convertHEICToJPEG decodes HEIC to JPEG. go-media is tried first, bounded for
// fit-mode sizes; ffmpeg is the fallback. Tile-grid iPhone HEIC cannot use ffmpeg
// -vf scale, so ffmpeg output is resized by CreatePreview.
func (s *Service) convertHEICToJPEG(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg is not available")
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	qualityStr := heicLegacyQualityString(previewSize)
	goQuality := ffmpeg.PreviewQualityFromLegacyQScale(qualityStr)
	maxW, maxH := heicDecodeBounds(previewSize)

	shadowCtx := context.WithoutCancel(ctx)
	// The shadow legacy result must match what production returns for this size:
	// ffmpeg's full decode, resized to the same fit box when go-media is bounded.
	legacy := func() ([]byte, bool) {
		data, err := s.ffmpegService.ConvertHEICToJPEG(shadowCtx, filePath, 0, 0, qualityStr)
		if err != nil || len(data) == 0 {
			return nil, false
		}
		if maxW > 0 {
			opts, err := getPreviewOptions(previewSize)
			if err != nil {
				return nil, false
			}
			resized, err := s.CreatePreview(shadowCtx, bytes.NewReader(data), 0, opts)
			if err != nil || len(resized) == 0 {
				return nil, false
			}
			return resized, true
		}
		return data, true
	}
	if data, ok := gomedia.HEICJPEG(filePath, maxW, maxH, goQuality, legacy); ok {
		return data, nil
	}

	if settings.Env.IsDevMode {
		logger.Debugf("go-media heic jpeg unavailable for %s; using ffmpeg", filePath)
	}
	return s.convertHEICToJPEGWithFFmpeg(ctx, filePath, previewSize)
}

// convertHEICToJPEGWithFFmpeg decodes HEIC to JPEG via ffmpeg when go-media
// cannot handle the file.
func (s *Service) convertHEICToJPEGWithFFmpeg(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg is not available")
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	qualityStr := heicLegacyQualityString(previewSize)
	return s.ffmpegService.ConvertHEICToJPEG(ctx, filePath, 0, 0, qualityStr)
}

// convertImageWithFFmpeg converts images to JPEG via go-media (camera RAW) or ffmpeg.
// Problematic JPEGs that Go's decoder cannot read use ffmpeg only; go-media does not
// treat standalone JPEG files as containers.
func (s *Service) convertImageWithFFmpeg(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg is not available for JPEG fallback")
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	width, height, quality := ffmpegImagePreviewBounds(previewSize)

	ext := strings.ToLower(filepath.Ext(filePath))
	if iteminfo.IsRawImage(ext) {
		goQuality := ffmpeg.PreviewQualityFromLegacyQScale(quality)
		shadowCtx := context.WithoutCancel(ctx)
		legacy := func() ([]byte, bool) {
			data, err := s.ffmpegService.ConvertImageToJPEG(shadowCtx, filePath, width, height, quality)
			return data, err == nil && len(data) > 0
		}
		if data, ok := gomedia.CameraRAWJPEG(filePath, width, height, goQuality, legacy); ok {
			return data, nil
		}
	}

	return s.ffmpegService.ConvertImageToJPEG(ctx, filePath, width, height, quality)
}

func ffmpegImagePreviewBounds(previewSize string) (width, height int, quality string) {
	switch previewSize {
	case "large":
		return 640, 640, "2"
	case "original":
		return 0, 0, "1"
	default:
		return 256, 256, "5"
	}
}
