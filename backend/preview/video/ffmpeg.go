package video

import (
	"os/exec"
	"strconv"
)

// GeneratePreviewImages generates preview images from a video using ffmpeg.
// videoPath: path to the input video file.
// outputPathPattern: path pattern where the generated preview images will be saved (e.g., "/tmp/output_%03d.jpg").
// numImages: number of preview images to generate.
func GeneratePreviewImages(ffmpegPath, videoPath, outputPathPattern string, numImages int) error {
	cmd := exec.Command(
		ffmpegPath,
		"-i", videoPath,
		"-vf", "thumbnail,scale=640:360",
		"-frames:v", strconv.Itoa(numImages),
		"-vsync", "vfr",
		outputPathPattern,
	)

	// Optional: capture stdout/stderr if needed for debugging
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	return cmd.Run()
}
