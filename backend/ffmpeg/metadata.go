package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

var blockedMediaPathPrefixes = []string{
	"pipe:",
	"concat:",
	"subfile:",
	"crypto:",
	"data:",
}

// resolveLocalMediaPath restricts ffprobe to absolute local files (no remote protocols).
func resolveLocalMediaPath(mediaPath string) (string, os.FileInfo, error) {
	trimmed := strings.TrimSpace(mediaPath)
	if trimmed == "" {
		return "", nil, fmt.Errorf("media path is empty")
	}
	if strings.Contains(trimmed, "://") {
		return "", nil, fmt.Errorf("remote media URLs are not allowed")
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range blockedMediaPathPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return "", nil, fmt.Errorf("remote media protocols are not allowed")
		}
	}

	clean := filepath.Clean(trimmed)
	if !filepath.IsAbs(clean) {
		return "", nil, fmt.Errorf("media path must be absolute")
	}

	info, err := os.Stat(clean)
	if err != nil {
		return "", nil, fmt.Errorf("failed to stat file: %w", err)
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("media path is a directory")
	}
	return clean, info, nil
}

// FileProbeInfo holds ffprobe results for a local media file.
type FileProbeInfo struct {
	Duration   float64
	VideoCodec string
	AudioCodec string
	FormatName string
}

// ProbeFile extracts duration and codec info from a media file.
func (s *Service) ProbeFile(ctx context.Context, mediaPath string) (*FileProbeInfo, error) {
	if s == nil || s.inner == nil {
		return nil, fmt.Errorf("ffmpeg service not available")
	}

	localPath, _, err := resolveLocalMediaPath(mediaPath)
	if err != nil {
		return nil, err
	}

	if err = s.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.Release()

	info, err := s.inner.ProbeStream(ctx, goffmpeg.ProbeStreamOptions{
		URL:        localPath,
		StreamType: goffmpeg.StreamFile,
		Timeout:    30 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	return &FileProbeInfo{
		Duration:   info.Duration,
		VideoCodec: info.VideoCodec,
		AudioCodec: info.AudioCodec,
		FormatName: info.FormatName,
	}, nil
}

// GetMediaDuration extracts the duration of a media file in seconds.
func (s *Service) GetMediaDuration(ctx context.Context, mediaPath string) (float64, error) {
	if s == nil || s.inner == nil {
		return 0, fmt.Errorf("ffmpeg service not available")
	}

	localPath, fileInfo, err := resolveLocalMediaPath(mediaPath)
	if err != nil {
		return 0, err
	}
	cacheKey := fmt.Sprintf("%s:%d", localPath, fileInfo.ModTime().Unix())
	if duration, found := MetadataCache.Get(cacheKey); found {
		return duration, nil
	}

	if err = s.Acquire(ctx); err != nil {
		return 0, err
	}
	defer s.Release()

	duration, err := s.inner.GetMediaDuration(ctx, localPath)
	if err != nil {
		return 0, err
	}

	MetadataCache.Set(cacheKey, duration)
	return duration, nil
}
