package ffmpeg

import (
	"context"
	"fmt"
	"io"
	"os"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
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
	cacheKey := fmt.Sprintf("probe:%s:%d", path, fileInfo.ModTime().Unix())
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

// FMP4StreamCopy remuxes compatible streams to fragmented MP4 on w.
func (s *Service) FMP4StreamCopy(ctx context.Context, w io.Writer, path string) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()

	return s.inner.FMP4StreamCopy(ctx, w, ops.FMP4StreamCopyOptions{
		Input: ops.InputSource{URL: path, StreamType: probe.StreamFile},
	})
}

// FMP4Transcode re-encodes input to browser-safe fragmented MP4 on w.
func (s *Service) FMP4Transcode(ctx context.Context, w io.Writer, path string, decode encode.VideoDecodeProfile, profile encode.VideoProfile, maxHeight int) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()

	return s.inner.FMP4Transcode(ctx, w, ops.FMP4TranscodeOptions{
		Input:      ops.InputSource{URL: path, StreamType: probe.StreamFile},
		Decode:     decode,
		Profile:    profile,
		AudioCodec: "aac",
		MaxHeight:  maxHeight,
	})
}
