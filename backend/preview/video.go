package preview

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// outputPath: path where the generated preview image will be saved (e.g., "/tmp/preview.jpg").
// seekTime: how many seconds into the video to seek before capturing the frame.
func (s *Service) GenerateVideoPreview(videoPath, outputPath string, percentageSeek int) error {
	// Step 1: Get video duration from the container format
	probeCmd := exec.Command(
		s.ffprobePath,
		"-v", "error",
		// Use format=duration for better compatibility
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	var probeOut bytes.Buffer
	probeCmd.Stdout = &probeOut
	if s.debug {
		probeCmd.Stderr = os.Stderr
	}
	if err := probeCmd.Run(); err != nil {
		logger.Errorf("ffprobe command failed on file '%v' : %v", videoPath, err)
		return fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(probeOut.String())
	if durationStr == "" || durationStr == "N/A" {
		logger.Errorf("could not determine video duration for file '%v' using duration info '%v'", videoPath, durationStr)
		return fmt.Errorf("could not determine video duration")
	}

	durationFloat, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		// The original error you saw would be caught here if "N/A" was still the output
		return fmt.Errorf("invalid duration: %v", err)
	}

	if durationFloat <= 0 {
		return fmt.Errorf("video duration must be positive")
	}

	// The rest of your function remains the same...
	// Step 2: Get the duration of the video in whole seconds
	duration := int(durationFloat)

	// Step 3: Calculate seek time based on percentageSeek (percentage value)
	seekSeconds := duration * percentageSeek / 100

	// Step 4: Convert seekSeconds to string for ffmpeg command
	seekTime := strconv.Itoa(seekSeconds)
	// Step 5: Extract frame at seek time
	cmd := exec.Command(
		s.ffmpegPath,
		"-ss", seekTime,
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "10",
		"-y", // overwrite output
		outputPath,
	)

	if s.debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	return cmd.Run()
}
