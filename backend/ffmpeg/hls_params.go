package ffmpeg

import (
	"context"
	"fmt"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

// HLSNeedsFullVideoTranscode reports whether video must be re-encoded for the profile.
func HLSNeedsFullVideoTranscode(info StreamInfo, mode HLSTranscodeProfile, maxHeight int) bool {
	return goffmpeg.NeedsFullVideoTranscode(info, hlsPipelineOptions(mode, maxHeight))
}

// HLSUseVideoCopy selects H.264 stream-copy with audio transcode (quality path).
func HLSUseVideoCopy(info StreamInfo, mode HLSTranscodeProfile, maxHeight int) bool {
	return goffmpeg.UseVideoCopy(info, hlsPipelineOptions(mode, maxHeight))
}

// CanFMP4StreamCopy reports whether remux to fMP4 is possible.
func CanFMP4StreamCopy(info StreamInfo) bool {
	return goffmpeg.CanFMP4StreamCopy(info)
}

// CanH264VideoCopy is true when H.264 can be stream-copied and only audio needs transcoding.
func CanH264VideoCopy(info StreamInfo) bool {
	return goffmpeg.CanH264VideoCopy(info)
}

// BuildHLSSegmentParamsFast assembles encode params without probing fps (GOP uses default).
func BuildHLSSegmentParamsFast(in HLSSegmentBuildInput) HLSSegmentParams {
	return goffmpeg.BuildHLSSegmentParamsFast(in, onDemandDefaults())
}

// BuildHLSSegmentParams resolves GOP from fps when probeFPS is true.
func (s *Service) BuildHLSSegmentParams(ctx context.Context, path string, in HLSSegmentBuildInput, probeFPS bool) (HLSSegmentParams, error) {
	if s == nil || s.inner == nil {
		return HLSSegmentParams{}, fmt.Errorf("ffmpeg service not available")
	}
	if err := s.Acquire(ctx); err != nil {
		return HLSSegmentParams{}, err
	}
	defer s.Release()
	return s.inner.BuildHLSSegmentParams(ctx, path, in, onDemandDefaults(), probeFPS)
}

// BuildHLSSegmentBuildInput derives remux/copy/transcode flags from stream info and profile.
func BuildHLSSegmentBuildInput(info StreamInfo, mode HLSTranscodeProfile, maxHeight int) HLSSegmentBuildInput {
	return goffmpeg.BuildHLSSegmentBuildInput(info, hlsPipelineOptions(mode, maxHeight))
}

func hlsPipelineOptions(mode HLSTranscodeProfile, maxHeight int) goffmpeg.HLSPipelineOptions {
	force := false
	switch ParseHLSTranscodeProfile(string(mode)) {
	case HLSProfileOptimized, HLSProfileDataSaver:
		force = true
	}
	return goffmpeg.HLSPipelineOptions{
		ForceVideoTranscode: force,
		MaxHeight:           maxHeight,
	}
}
