package ffmpeg

import (
	"context"
	"fmt"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-ffmpeg/ops"
)

// StreamInfo is re-exported for transcoding callers.
type StreamInfo = goffmpeg.StreamInfo

// HLSPipelineOptions configures remux/copy/transcode path selection.
type HLSPipelineOptions = goffmpeg.HLSPipelineOptions

// HLSSegmentParams holds resolved encode/remux settings for one HLS session.
type HLSSegmentParams = goffmpeg.HLSSegmentParams

// HLSSegmentBuildInput describes remux/copy/transcode path selection for one file.
type HLSSegmentBuildInput = goffmpeg.HLSSegmentBuildInput

// HLSCacheIdentity describes inputs for a stable on-demand HLS disk cache key.
type HLSCacheIdentity = goffmpeg.HLSCacheIdentity

// VideoDecodeProfile selects input decode settings.
type VideoDecodeProfile = encode.VideoDecodeProfile

// VideoProfile holds encode settings.
type VideoProfile = encode.VideoProfile

// ProbeMediaStream probes a local media file for transcoding.
func (s *Service) ProbeMediaStream(ctx context.Context, path string) (StreamInfo, error) {
	if s == nil || s.inner == nil {
		return StreamInfo{}, errUnavailable()
	}
	return s.inner.ProbeFile(ctx, path)
}

// BuildHLSSegmentBuildInput derives remux/copy/transcode flags.
func BuildHLSSegmentBuildInput(info StreamInfo, opts HLSPipelineOptions) HLSSegmentBuildInput {
	return goffmpeg.BuildHLSSegmentBuildInput(info, opts)
}

// BuildHLSSegmentParams resolves GOP from fps when probeFPS is true.
func (s *Service) BuildHLSSegmentParams(ctx context.Context, path string, in HLSSegmentBuildInput, probeFPS bool) (HLSSegmentParams, error) {
	if s == nil || s.inner == nil {
		return HLSSegmentParams{}, errUnavailable()
	}
	defaults := goffmpeg.DefaultOnDemandHLSDefaults()
	return s.inner.BuildHLSSegmentParams(ctx, path, in, defaults, probeFPS)
}

// HLSCacheFingerprint returns a stable directory name for a transcode cache entry.
func HLSCacheFingerprint(id HLSCacheIdentity) string {
	return goffmpeg.HLSCacheFingerprint(id)
}

// DefaultOnDemandHLSDefaults returns on-demand segment defaults.
func DefaultOnDemandHLSDefaults() goffmpeg.OnDemandHLSDefaults {
	return goffmpeg.DefaultOnDemandHLSDefaults()
}

// PrepareHLSMediaTimeline normalizes keyframes and duration for HLS segment
// planning: container start_time is subtracted and corrupt keyframe indexes
// are filtered out.
func PrepareHLSMediaTimeline(keyframes []float64, durationSec, startTimeSec float64) (sanitizedKeyframes []float64, mediaDurationSec float64) {
	return goffmpeg.PrepareHLSMediaTimeline(keyframes, durationSec, startTimeSec)
}

// KeyframeSeekBefore returns the largest keyframe time <= sec, or 0 when none.
func KeyframeSeekBefore(keyframes []float64, sec float64) float64 {
	return goffmpeg.KeyframeSeekBefore(keyframes, sec)
}

// HLSDecodeProfileForOnDemand selects input decode for short on-demand HLS segments.
func HLSDecodeProfileForOnDemand(info StreamInfo) VideoDecodeProfile {
	return goffmpeg.HLSDecodeProfileForOnDemand(info)
}

// DefaultHLSVideoProfile returns safe H.264 transcode defaults.
func DefaultHLSVideoProfile(maxHeight int) VideoProfile {
	return goffmpeg.DefaultHLSVideoProfile(maxHeight)
}

// ProbeVideoKeyframeTimes returns keyframe presentation times in seconds.
func (s *Service) ProbeVideoKeyframeTimes(ctx context.Context, path string) ([]float64, error) {
	if s == nil || s.inner == nil {
		return nil, errUnavailable()
	}
	return s.inner.ProbeVideoKeyframeTimes(ctx, path)
}

// ProbeVideoKeyframeTimesBounded returns keyframe times from the first windowSec
// of the file only. Used as a cheap pre-flight to prove stream copy impossible
// before paying for the full-file keyframe demux.
func (s *Service) ProbeVideoKeyframeTimesBounded(ctx context.Context, path string, windowSec float64) ([]float64, error) {
	if s == nil || s.inner == nil {
		return nil, errUnavailable()
	}
	return s.inner.ProbeVideoKeyframeTimesBounded(ctx, path, windowSec)
}

func errUnavailable() error {
	return fmt.Errorf("ffmpeg service not available")
}

// InputSource describes ffmpeg input.
type InputSource = ops.InputSource

// StreamFile is the file stream type constant.
const StreamFile = goffmpeg.StreamFile
