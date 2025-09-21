package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// VideoService handles video preview operations with ffmpeg
type VideoService struct {
	ffmpegPath  string
	ffprobePath string
	debug       bool
	semaphore   chan struct{}
}

// NewVideoService creates a new video service instance
func NewVideoService(ffmpegPath, ffprobePath string, maxConcurrent int, debug bool) *VideoService {
	return &VideoService{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		debug:       debug,
		semaphore:   make(chan struct{}, maxConcurrent),
	}
}

func (s *VideoService) acquire(ctx context.Context) error {
	select {
	case s.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *VideoService) release() {
	<-s.semaphore
}

// GenerateVideoPreview generates a single preview image from a video using ffmpeg.
// videoPath: path to the input video file.
// outputPath: path where the generated preview image will be saved (e.g., "/tmp/preview.jpg").
// percentageSeek: percentage of video duration to seek to (0-100).
func (s *VideoService) GenerateVideoPreview(videoPath, outputPath string, percentageSeek int) ([]byte, error) {
	if err := s.acquire(context.Background()); err != nil {
		return nil, err
	}
	defer s.release()

	// Validate percentage parameter
	if percentageSeek < 0 || percentageSeek > 100 {
		percentageSeek = 10 // Default to 10% if invalid
	}

	// Step 1: Get video duration from the container format
	probeCmd := exec.Command(
		s.ffprobePath,
		"-v", "error",
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
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(probeOut.String())
	if durationStr == "" || durationStr == "N/A" {
		logger.Errorf("could not determine video duration for file '%v' using duration info '%v'", videoPath, durationStr)
		return nil, fmt.Errorf("could not determine video duration")
	}

	durationFloat, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid duration: %v", err)
	}

	if durationFloat <= 0 {
		return nil, fmt.Errorf("video duration must be positive")
	}

	// Step 2: Calculate seek time with higher precision
	seekTime := durationFloat * float64(percentageSeek) / 100.0
	seekTimeStr := strconv.FormatFloat(seekTime, 'f', 3, 64) // 3 decimal places precision

	// Step 3: Extract frame at seek time with optimized parameters
	cmd := exec.Command(
		s.ffmpegPath,
		"-ss", seekTimeStr, // Use precise seek time
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "10", // Good quality/size balance
		"-f", "image2", // Explicitly specify image format
		"-y", // overwrite output
		outputPath,
	)

	if s.debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err = cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg command failed on file '%v' : %w", videoPath, err)
	}
	return os.ReadFile(outputPath)
}

// GenerateVideoPreviewStreaming generates a video preview and streams it directly to a writer
// This is more memory efficient for large previews as it doesn't load the entire file into memory
func (s *VideoService) GenerateVideoPreviewStreaming(videoPath string, percentageSeek int, writer io.Writer) error {
	if err := s.acquire(context.Background()); err != nil {
		return err
	}
	defer s.release()

	// Validate percentage parameter
	if percentageSeek < 0 || percentageSeek > 100 {
		percentageSeek = 10 // Default to 10% if invalid
	}

	// Step 1: Get video duration from the container format
	probeCmd := exec.Command(
		s.ffprobePath,
		"-v", "error",
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
		return fmt.Errorf("invalid duration: %v", err)
	}

	if durationFloat <= 0 {
		return fmt.Errorf("video duration must be positive")
	}

	// Step 2: Calculate seek time with higher precision
	seekTime := durationFloat * float64(percentageSeek) / 100.0
	seekTimeStr := strconv.FormatFloat(seekTime, 'f', 3, 64) // 3 decimal places precision

	// Step 3: Extract frame and stream directly to writer
	cmd := exec.Command(
		s.ffmpegPath,
		"-ss", seekTimeStr, // Use precise seek time
		"-i", videoPath,
		"-frames:v", "1",
		"-q:v", "10", // Good quality/size balance
		"-f", "image2", // Explicitly specify image format
		"-vcodec", "mjpeg", // Use MJPEG codec for better compression
		"-", // Output to stdout
	)

	cmd.Stdout = writer
	if s.debug {
		cmd.Stderr = os.Stderr
	}

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("ffmpeg command failed on file '%v' : %w", videoPath, err)
	}
	return nil
}

// GetVideoDuration extracts the duration of a video file using ffprobe
func GetVideoDuration(ffprobePath string, videoPath string) (float64, error) {
	cmd := exec.Command(ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	if durationStr == "" || durationStr == "N/A" {
		return 0, fmt.Errorf("could not determine video duration")
	}

	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %v", err)
	}

	return duration, nil
}
