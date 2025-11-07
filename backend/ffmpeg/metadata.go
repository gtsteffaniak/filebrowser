package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// GetMediaDuration extracts the duration of a media file (audio or video) using ffprobe
// This method respects the concurrency limits, uses caching, and gracefully handles errors
// Cache key is based on file path + mod time for automatic cache invalidation on file changes
func (s *FFmpegService) GetMediaDuration(ctx context.Context, mediaPath string) (float64, error) {
	// Get file info for cache key (path + mod time)
	fileInfo, err := os.Stat(mediaPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	duration := float64(0)
	found := false
	// Create cache key from path and modification time
	cacheKey := fmt.Sprintf("%s:%d", mediaPath, fileInfo.ModTime().Unix())
	// Check cache first
	if duration, found = MetadataCache.Get(cacheKey); found {
		return duration, nil
	}
	defer MetadataCache.Set(cacheKey, duration)
	// Acquire semaphore slot for concurrency control
	if err = s.Acquire(ctx); err != nil {
		return 0, err
	}
	defer s.Release()

	// Check if context is cancelled before starting
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}

	cmd := exec.CommandContext(ctx,
		s.ffprobePath,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		mediaPath,
	)

	output, err := cmd.Output()
	if err != nil {
		// Silently return error - ffprobe might not be installed
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	if durationStr == "" || durationStr == "N/A" {
		return 0, fmt.Errorf("could not determine media duration")
	}

	duration, err = strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid duration: %v", err)
	}

	return duration, nil
}
