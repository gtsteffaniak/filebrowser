package preview

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// outputPath: path where the generated preview image will be saved (e.g., "/tmp/preview.jpg").
// seekTime: how many seconds into the video to seek before capturing the frame.
func (s *Service) GenerateVideoPreview(videoPath, outputPath string) error {
	// Step 1: Get video stream duration (v:0)
	probeCmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	var probeOut bytes.Buffer
	probeCmd.Stdout = &probeOut
	probeCmd.Stderr = os.Stderr

	if err := probeCmd.Run(); err != nil {
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(probeOut.String())
	durationFloat, err := strconv.ParseFloat(durationStr, 64)
	if err != nil || durationFloat <= 0 {
		return fmt.Errorf("invalid duration: %v", err)
	}

	// Step 2: Truncate and compute 10% of video length
	duration := int(durationFloat)
	seekSeconds := duration / 10
	seekTime := strconv.Itoa(seekSeconds)

	// Step 3: Extract frame at seek time
	cmd := exec.Command(
		s.ffmpegPath,
		"-ss", seekTime,
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "10",
		"-y", // overwrite output
		outputPath,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
