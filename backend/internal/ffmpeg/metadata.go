package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/mediacache"
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
	if strings.TrimSpace(mediaPath) == "" {
		return "", nil, fmt.Errorf("media path is empty")
	}
	if strings.Contains(mediaPath, "://") {
		return "", nil, fmt.Errorf("remote media URLs are not allowed")
	}
	lower := strings.ToLower(strings.TrimSpace(mediaPath))
	for _, prefix := range blockedMediaPathPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return "", nil, fmt.Errorf("remote media protocols are not allowed")
		}
	}

	clean := filepath.Clean(mediaPath)
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

// FileProbeInfo holds probe results for a local media file. It is the shared
// cache type, so go-media and ffprobe results are interchangeable.
type FileProbeInfo = mediacache.ProbeInfo

// ProbeFile extracts duration and codec info from a media file. Results are
// stored in the shared media cache, which the go-media probe also reads, unless
// ctx is mediacache.WithoutCache.
func (s *Service) ProbeFile(ctx context.Context, mediaPath string) (*FileProbeInfo, error) {
	if s == nil || s.inner == nil {
		return nil, fmt.Errorf("ffmpeg service not available")
	}

	localPath, fileInfo, err := resolveLocalMediaPath(mediaPath)
	if err != nil {
		return nil, err
	}
	useCache := !mediacache.Bypassed(ctx)
	cacheKey := mediacache.FromFileInfo(localPath, fileInfo).String()
	if useCache {
		if cached, ok := mediacache.Probes.Get(cacheKey); ok {
			return &cached, nil
		}
	}

	info, err := s.inner.ProbeStream(ctx, goffmpeg.ProbeStreamOptions{
		URL:        localPath,
		StreamType: goffmpeg.StreamFile,
		Timeout:    30 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	res := FileProbeInfo{
		Duration:   info.Duration,
		VideoCodec: info.VideoCodec,
		AudioCodec: info.AudioCodec,
		FormatName: info.FormatName,
	}
	if useCache {
		mediacache.Probes.Set(cacheKey, res)
	}
	return &res, nil
}

// GetMediaDuration extracts the duration of a media file in seconds. Results
// are stored in the shared media cache unless ctx is mediacache.WithoutCache.
func (s *Service) GetMediaDuration(ctx context.Context, mediaPath string) (float64, error) {
	if s == nil || s.inner == nil {
		return 0, fmt.Errorf("ffmpeg service not available")
	}

	localPath, fileInfo, err := resolveLocalMediaPath(mediaPath)
	if err != nil {
		return 0, err
	}
	useCache := !mediacache.Bypassed(ctx)
	cacheKey := mediacache.FromFileInfo(localPath, fileInfo).String()
	if useCache {
		if duration, found := mediacache.Durations.Get(cacheKey); found {
			return duration, nil
		}
	}

	duration, err := s.inner.GetMediaDuration(ctx, localPath)
	if err != nil {
		return 0, err
	}

	if useCache {
		mediacache.Durations.Set(cacheKey, duration)
	}
	return duration, nil
}
