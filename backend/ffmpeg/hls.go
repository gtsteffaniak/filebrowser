package ffmpeg

import (
	"context"
	"fmt"
	"io"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

// HLSSegmentDurationSec is deprecated; use SegmentDurationSec().
const HLSSegmentDurationSec = DefaultHLSSegmentDurationSec

// HLSSegmentParams holds resolved encode/remux settings for one HLS session.
type HLSSegmentParams = goffmpeg.HLSSegmentParams

// HLSSegmentBuildInput describes remux/copy/transcode path selection for one file.
type HLSSegmentBuildInput = goffmpeg.HLSSegmentBuildInput

func onDemandDefaults() goffmpeg.OnDemandHLSDefaults {
	cfg := ActiveHLSConfig().Normalized()
	return goffmpeg.OnDemandHLSDefaults{
		SegmentDurationSec: cfg.SegmentDurationSec,
		DefaultGOP:         cfg.DefaultGOP,
	}
}

// SanitizeHLSKeyframes filters spurious keyframe probes from corrupt indexes.
func SanitizeHLSKeyframes(keyframes []float64, durationSec float64) []float64 {
	return goffmpeg.SanitizeHLSKeyframes(keyframes, durationSec)
}

// BuildHLSSegmentTimeline returns segment start times and durations in seconds.
func BuildHLSSegmentTimeline(durationSec float64, keyframes []float64) (starts, durations []float64) {
	return goffmpeg.BuildHLSSegmentTimeline(durationSec, keyframes, SegmentDurationSec())
}

// BuildHLSSegmentOptions builds go-ffmpeg segment options for segment index n.
func BuildHLSSegmentOptions(path string, index int, params HLSSegmentParams, starts, durations []float64, keyframeTimeline bool, keyframeSeekTimes []float64) goffmpeg.HLSSegmentOptions {
	return goffmpeg.BuildHLSSegmentOptions(path, index, params, starts, durations, keyframeTimeline, keyframeSeekTimes, SegmentDurationSec())
}

// KeyframeSeekBefore returns the largest keyframe time <= sec, or 0 when none.
func KeyframeSeekBefore(keyframes []float64, sec float64) float64 {
	return goffmpeg.KeyframeSeekBefore(keyframes, sec)
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
