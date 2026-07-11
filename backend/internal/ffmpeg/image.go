package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/imagemeta"
	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

// GetImageOrientation extracts the EXIF orientation from an image file.
func (s *Service) GetImageOrientation(imagePath string) (string, error) {
	if orientation := imagemeta.GetOrientation(context.Background(), imagePath); orientation != "" {
		return orientation, nil
	}
	return "Horizontal (normal)", nil
}

// GetHEICOrientation returns the display orientation for HEIC/HEIF files.
func (s *Service) GetHEICOrientation(heicPath string) (string, error) {
	return s.GetImageOrientation(heicPath)
}

// GetOrientationFilter converts EXIF orientation to an FFmpeg filter suffix.
func (s *Service) GetOrientationFilter(orientation string) string {
	switch orientation {
	case "Rotate 90 CW", "Right-top":
		return ",transpose=1"
	case "Rotate 180", "Bottom-right":
		return ",transpose=1,transpose=1"
	case "Rotate 270 CW", "Left-bottom":
		return ",transpose=2"
	case "Mirror horizontal", "Top-right":
		return ",hflip"
	case "Mirror vertical", "Bottom-left":
		return ",vflip"
	case "Mirror horizontal and rotate 270 CW", "Right-bottom":
		return ",transpose=0"
	case "Mirror horizontal and rotate 90 CW", "Left-top":
		return ",transpose=3"
	case "Horizontal (normal)", "Top-left":
		return ""
	case "1":
		return ""
	case "2":
		return ",hflip"
	case "3":
		return ",transpose=1,transpose=1"
	case "4":
		return ",vflip"
	case "5":
		return ",transpose=3"
	case "6":
		return ",transpose=1"
	case "7":
		return ",transpose=0"
	case "8":
		return ",transpose=2"
	default:
		return ""
	}
}

// ConvertHEICToJPEG decodes HEIC/HEIF to JPEG. Tile-grid iPhone HEIC cannot use ffmpeg -vf,
// so decode is filter-free and any remaining orientation is applied in Go afterward.
func (s *Service) ConvertHEICToJPEG(ctx context.Context, heicPath string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	if s == nil || s.inner == nil {
		return nil, fmt.Errorf("ffmpeg service not available")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	orientation, err := s.GetHEICOrientation(heicPath)
	if err != nil {
		orientation = "Horizontal (normal)"
	}

	jpegBytes, err := s.runHEICConversion(ctx, heicPath, targetWidth, targetHeight, quality, "")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg image conversion failed: %w", err)
	}

	displayRotation, displayKnown := s.GetHEICDisplayMatrixRotation(ctx, heicPath)
	if orientationNeedsPostCorrection(orientation, displayRotation, displayKnown) {
		jpegBytes = applyOrientationToJPEG(jpegBytes, orientation)
	}
	return jpegBytes, nil
}

// ConvertImageToJPEG converts any supported image to JPEG using ffmpeg.
func (s *Service) ConvertImageToJPEG(ctx context.Context, imagePath string, targetWidth, targetHeight int, quality string) ([]byte, error) {
	return s.convertToJPEG(ctx, imagePath, targetWidth, targetHeight, quality, true)
}

func (s *Service) convertToJPEG(ctx context.Context, inputPath string, targetWidth, targetHeight int, quality string, applyOrientation bool) ([]byte, error) {
	if s == nil || s.inner == nil {
		return nil, fmt.Errorf("ffmpeg service not available")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	orientationFilter := ""
	if applyOrientation {
		orientation, err := s.GetImageOrientation(inputPath)
		if err != nil {
			orientation = "Horizontal (normal)"
		}
		orientationFilter = strings.TrimPrefix(s.GetOrientationFilter(orientation), ",")
	}

	jpegBytes, err := s.runHEICConversion(ctx, inputPath, targetWidth, targetHeight, quality, orientationFilter)
	if err == nil {
		return jpegBytes, nil
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if orientationFilter != "" {
		if jpegBytes, retryErr := s.runHEICConversion(ctx, inputPath, targetWidth, targetHeight, quality, ""); retryErr == nil {
			return jpegBytes, nil
		}
	}
	return nil, fmt.Errorf("ffmpeg image conversion failed: %w", err)
}

func (s *Service) runHEICConversion(ctx context.Context, inputPath string, targetWidth, targetHeight int, quality, orientationFilter string) ([]byte, error) {
	outputDir := s.cacheDir
	if err := os.MkdirAll(outputDir, fileutils.PermDir); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	tmp, err := os.CreateTemp(outputDir, "ffmpeg_convert_*.jpg")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	outputFile := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(outputFile)

	opts := goffmpeg.ConvertHEICOptions{
		InputPath:         inputPath,
		OutputPath:        outputFile,
		Width:             targetWidth,
		Height:            targetHeight,
		Quality:           parseQuality(quality),
		OrientationFilter: orientationFilter,
	}
	if err := s.inner.ConvertHEIC(ctx, opts); err != nil {
		return nil, err
	}
	return os.ReadFile(outputFile)
}

func parseQuality(quality string) int {
	n, err := strconv.Atoi(quality)
	if err != nil || n < 1 {
		return 5
	}
	if n > 10 {
		return 10
	}
	return n
}
