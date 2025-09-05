package preview

import (
	"context"

	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
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

	// Create a temporary video service for this operation
	videoSvc := ffmpeg.NewVideoService(s.ffmpegPath, s.ffprobePath, 1, s.debug)
	return videoSvc.GenerateVideoPreview(videoPath, outputPath, percentageSeek)
}
