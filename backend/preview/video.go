package preview

import (
	"os"
	"os/exec"
	"strconv"
)

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// outputPath: path where the generated preview image will be saved (e.g., "/tmp/preview.jpg").
// seekTime: how many seconds into the video to seek before capturing the frame.
func (s *Service) GenerateVideoPreview(videoPath, outputPath string, seekTime int) error {
	cmd := exec.Command(
		s.ffmpegPath,
		"-ss", strconv.Itoa(seekTime), // seek to a better frame
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "10", // quality 1 is best, 31 is worst
		outputPath,
	)

	// Optional: capture stdout/stderr for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
