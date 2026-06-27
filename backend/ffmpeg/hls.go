package ffmpeg

import (
	"context"
	"fmt"
	"io"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/gtsteffaniak/go-ffmpeg/probe"
)

const HLSSegmentDurationSec = 4.0

// HLSSegmentParams holds resolved encode/remux settings for one HLS session.
type HLSSegmentParams struct {
	Remux     bool
	VideoCopy bool
	Decode    encode.VideoDecodeProfile
	Profile   encode.VideoProfile
	MaxHeight int
	GOP       int
}

// ProbeVideoFPS returns average video frame rate for GOP sizing.
func (s *Service) ProbeVideoFPS(ctx context.Context, path string) (float64, error) {
	if s == nil || s.inner == nil {
		return 30, fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return 30, err
	}
	defer s.Release()
	return s.inner.ProbeVideoFPS(ctx, path)
}

// ProbeVideoKeyframeTimes returns keyframe PTS values in seconds for segment alignment.
func (s *Service) ProbeVideoKeyframeTimes(ctx context.Context, path string) ([]float64, error) {
	if s == nil || s.inner == nil {
		return nil, fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.Release()
	return s.inner.ProbeVideoKeyframeTimes(ctx, path)
}

// BuildHLSSegmentOptions builds go-ffmpeg segment options for segment index n.
func BuildHLSSegmentOptions(path string, index int, params HLSSegmentParams, starts, durations []float64) goffmpeg.HLSSegmentOptions {
	startSec := float64(index) * HLSSegmentDurationSec
	durSec := HLSSegmentDurationSec
	if index >= 0 && index < len(starts) {
		startSec = starts[index]
	}
	if index >= 0 && index < len(durations) {
		durSec = durations[index]
	}
	return goffmpeg.HLSSegmentOptions{
		Input:       ops.InputSource{URL: path, StreamType: probe.StreamFile},
		StartSec:    startSec,
		DurationSec: durSec,
		Decode:      params.Decode,
		Profile:     params.Profile,
		MaxHeight:   params.MaxHeight,
		Remux:       params.Remux,
		VideoCopy:   params.VideoCopy,
		GOP:         params.GOP,
	}
}

// HLSSegment generates a self-contained MPEG-TS segment for full re-encode HLS.
func (s *Service) HLSSegment(ctx context.Context, opts goffmpeg.HLSSegmentOptions) ([]byte, error) {
	if s == nil || s.inner == nil {
		return nil, fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return nil, err
	}
	defer s.Release()
	return s.inner.HLSSegment(ctx, opts)
}

// HLSInitAndSegment generates init + media for segment 0.
func (s *Service) HLSInitAndSegment(ctx context.Context, opts goffmpeg.HLSSegmentOptions) (init, media []byte, err error) {
	if s == nil || s.inner == nil {
		return nil, nil, fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return nil, nil, err
	}
	defer s.Release()
	return s.inner.HLSInitAndSegment(ctx, opts)
}

// HLSSegmentMedia writes a media-only fragment for segment index n.
func (s *Service) HLSSegmentMedia(ctx context.Context, w io.Writer, opts goffmpeg.HLSSegmentOptions) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return err
	}
	defer s.Release()
	return s.inner.HLSSegmentMedia(ctx, w, opts)
}
