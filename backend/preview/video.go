package preview

import (
	"context"
	"fmt"
)

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// outputPath: path where the generated preview image will be saved (e.g., "/tmp/preview.jpg").
// seekTime: how many seconds into the video to seek before capturing the frame.
func (s *Service) GenerateVideoPreview(videoPath, outputPath string, percentageSeek int) ([]byte, error) {
	if err := s.acquire(context.Background()); err != nil {
		return nil, err
	}
	defer s.release()

	// Use the shared video service
	if s.videoService == nil {
		return nil, fmt.Errorf("video service not available")
	}
	return s.videoService.GenerateVideoPreview(videoPath, outputPath, percentageSeek)
}
