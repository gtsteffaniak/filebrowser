package preview

import (
	"context"
	"fmt"
)

// convertHEICToJPEGWithFFmpeg decodes HEIC to JPEG at full resolution via ffmpeg.
// Tile-grid iPhone HEIC cannot use ffmpeg -vf scale; resize is done later by CreatePreview.
// Passing non-zero width/height would add -vf scale and fail on those files.
func (s *Service) convertHEICToJPEGWithFFmpeg(ctx context.Context, filePath string, previewSize string) ([]byte, error) {
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg is not available")
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if err := s.ffmpegService.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.ffmpegService.Release()

	var quality string
	switch previewSize {
	case "large":
		quality = "2"
	case "original":
		quality = "1"
	default:
		quality = "5"
	}

	return s.ffmpegService.ConvertHEICToJPEG(ctx, filePath, 0, 0, quality)
}

// convertImageWithFFmpeg converts any image file (including problematic JPEGs) to resized JPEG using FFmpeg.
// This is used as a fallback for JPEG files that Go's standard decoder can't handle (extended sequential, etc.)
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

	return s.ffmpegService.ConvertImageToJPEG(ctx, filePath, width, height, quality)
}
