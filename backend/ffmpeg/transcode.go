package ffmpeg

import (
	"context"
	"fmt"
	"os"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

// StreamInfo holds ffprobe results for a media file.
type StreamInfo = goffmpeg.StreamInfo

// ProbeFile probes a local media file and returns stream metadata.
func (s *Service) ProbeFile(ctx context.Context, path string) (StreamInfo, error) {
	if s == nil || s.inner == nil {
		return StreamInfo{}, fmt.Errorf("ffmpeg service not available")
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		return StreamInfo{}, fmt.Errorf("failed to stat file: %w", err)
	}
	cacheKey := fmt.Sprintf("probe:%s:%d:%d", path, fileInfo.ModTime().UnixNano(), fileInfo.Size())
	if info, ok := ProbeCache.Get(cacheKey); ok {
		return info, nil
	}

	if err = s.Acquire(ctx); err != nil {
		return StreamInfo{}, err
	}
	defer s.Release()

	info, err := s.inner.ProbeStream(ctx, goffmpeg.ProbeStreamOptions{
		URL:        path,
		StreamType: probe.StreamFile,
	})
	if err != nil {
		return info, err
	}

	ProbeCache.Set(cacheKey, info)
	return info, nil
}
