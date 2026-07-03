package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

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

	if err := s.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.Release()

	info, err := s.inner.ProbeStream(ctx, goffmpeg.ProbeStreamOptions{
		URL:        mediaPath,
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

	fileInfo, err := os.Stat(mediaPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	cacheKey := fmt.Sprintf("%s:%d", mediaPath, fileInfo.ModTime().Unix())
	if duration, found := MetadataCache.Get(cacheKey); found {
		return duration, nil
	}

	if err = s.Acquire(ctx); err != nil {
		return 0, err
	}
	defer s.Release()

	duration, err := s.inner.GetMediaDuration(ctx, mediaPath)
	if err != nil {
		return 0, err
	}

	MetadataCache.Set(cacheKey, duration)
	return duration, nil
}
