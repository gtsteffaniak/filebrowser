package ffmpeg

import (
	"context"
	"fmt"
	"os"
)

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
