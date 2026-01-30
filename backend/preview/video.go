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
	err := s.ffmpegService.GenerateVideoPreviewStreaming(ctx, videoPath, percentageSeek, &buf)
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
