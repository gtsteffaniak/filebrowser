package preview

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"github.com/gtsteffaniak/go-logger/logger"
)

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// percentageSeek: percentage of video duration to seek to (0–100).
// Returns: JPEG image bytes.
func (s *Service) GenerateVideoPreview(ctx context.Context, videoPath string, percentageSeek int) ([]byte, error) {
	if s.ffmpegService == nil {
		logger.Errorf("FFmpeg service not available for file '%s'", videoPath)
		return nil, fmt.Errorf("FFmpeg service not available")
	}

	// Validate file exists before processing
	if _, err := os.Stat(videoPath); err != nil {
		logger.Errorf("Video file does not exist or is not accessible: '%s': %v", videoPath, err)
		return nil, fmt.Errorf("video file not accessible: %w", err)
	}

	var buf bytes.Buffer
	err := s.ffmpegService.VideoPreview(ctx, &buf, videoPath, percentageSeek)
	if err != nil {
		return nil, err
	}

	previewBytes := buf.Bytes()
	if len(previewBytes) == 0 {
		logger.Errorf("FFmpeg service returned empty result for '%s'", videoPath)
		return nil, fmt.Errorf("video preview generation returned empty result")
	}

	return previewBytes, nil
}


// GenerateVideoPreviewAtTime generates a preview image at an absolute timestamp.
func (s *Service) GenerateVideoPreviewAtTime(ctx context.Context, videoPath string, seekSec float64) ([]byte, error) {
	if s.ffmpegService == nil {
		return nil, fmt.Errorf("FFmpeg service not available")
	}
	if _, err := os.Stat(videoPath); err != nil {
		return nil, fmt.Errorf("video file not accessible: %w", err)
	}
	var buf bytes.Buffer
	if err := s.ffmpegService.VideoPreviewAtTime(ctx, &buf, videoPath, seekSec); err != nil {
		return nil, err
	}
	if len(buf.Bytes()) == 0 {
		return nil, fmt.Errorf("video preview generation returned empty result")
	}
	return buf.Bytes(), nil
}
