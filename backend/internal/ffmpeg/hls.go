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

// HLSContinuousOptions configures a long-running ffmpeg HLS job.
type HLSContinuousOptions = goffmpeg.HLSContinuousOptions

// HLSContinuousJob runs ffmpeg until EOF, cancellation, or error.
type HLSContinuousJob = goffmpeg.HLSContinuousJob

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

// BuildHLSSegmentTimeline returns segment start times and durations.
func BuildHLSSegmentTimeline(durationSec float64, keyframes []float64, segmentDurationSec float64) (starts, durations []float64) {
	return goffmpeg.BuildHLSSegmentTimeline(durationSec, keyframes, segmentDurationSec)
}

// SanitizeHLSKeyframes filters spurious keyframe probes.
func SanitizeHLSKeyframes(keyframes []float64, durationSec float64) []float64 {
	return goffmpeg.SanitizeHLSKeyframes(keyframes, durationSec)
}

// NeedsFullVideoTranscode reports whether video must be re-encoded.
func NeedsFullVideoTranscode(info StreamInfo, opts HLSPipelineOptions) bool {
	return goffmpeg.NeedsFullVideoTranscode(info, opts)
}

// HLSDecodeProfileForOnDemand selects input decode for short on-demand HLS segments.
func HLSDecodeProfileForOnDemand(info StreamInfo) VideoDecodeProfile {
	return goffmpeg.HLSDecodeProfileForOnDemand(info)
}

// DefaultHLSVideoProfile returns safe H.264 transcode defaults.
func DefaultHLSVideoProfile(maxHeight int) VideoProfile {
	return goffmpeg.DefaultHLSVideoProfile(maxHeight)
}

// DescribeHLSSegmentPlan summarizes the encode path for logging.
func (s *Service) DescribeHLSSegmentPlan(params HLSSegmentParams) string {
	if s == nil || s.inner == nil {
		return ""
	}
	return s.inner.DescribeHLSSegmentPlan(params)
}

// ProbeVideoKeyframeTimes returns keyframe presentation times in seconds.
func (s *Service) ProbeVideoKeyframeTimes(ctx context.Context, path string) ([]float64, error) {
	if s == nil || s.inner == nil {
		return nil, errUnavailable()
	}
	return s.inner.ProbeVideoKeyframeTimes(ctx, path)
}

// StartHLSContinuous launches ffmpeg -f hls writing fMP4 segments to disk.
func (s *Service) StartHLSContinuous(ctx context.Context, opts HLSContinuousOptions) (*HLSContinuousJob, error) {
	if s == nil || s.inner == nil {
		return nil, errUnavailable()
	}
	return s.inner.StartHLSContinuous(ctx, opts)
}

func errUnavailable() error {
	return fmt.Errorf("ffmpeg service not available")
}

// InputSource describes ffmpeg input.
type InputSource = ops.InputSource

// StreamFile is the file stream type constant.
const StreamFile = goffmpeg.StreamFile

// HLSContinuousCacheFill pacing mode.
const HLSContinuousCacheFill = goffmpeg.HLSContinuousCacheFill
