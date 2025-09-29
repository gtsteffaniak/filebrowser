package preview

import (
	"bytes"
	"context"
	"fmt"
)

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// percentageSeek: percentage of video duration to seek to (0–100).
// Returns: JPEG image bytes.
func (s *Service) GenerateVideoPreview(ctx context.Context, videoPath string, percentageSeek int) ([]byte, error) {
	if err := s.acquire(ctx); err != nil {
		return nil, err
	}
	defer s.release()

	if s.videoService == nil {
		return nil, fmt.Errorf("video service not available")
	}

	var buf bytes.Buffer
	err := s.videoService.GenerateVideoPreviewStreaming(ctx, videoPath, percentageSeek, &buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
