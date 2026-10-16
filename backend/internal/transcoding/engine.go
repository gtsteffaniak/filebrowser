package transcoding

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/encode"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// MediaEngine adapts the ffmpeg service for HLS transcoding.
type MediaEngine struct{}

// ContinuousJob is a running ffmpeg HLS encode.
type ContinuousJob interface {
	Cancel()
	Wait() error
}

// Engine performs probe/plan/start operations for tests and production.
type Engine interface {
	Available() bool
	BuildPlan(ctx context.Context, realPath string, profile Profile) (hlsPlan, error)
	StartContinuous(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, startSec float64) (ContinuousJob, error)
}

func NewMediaEngine() *MediaEngine {
	return &MediaEngine{}
}

var _ Engine = (*MediaEngine)(nil)

func (e *MediaEngine) Available() bool {
	return ffmpeg.Enabled() && settings.TranscodeEnabled()
}

func (e *MediaEngine) ProbeFile(ctx context.Context, path string) (ffmpeg.StreamInfo, error) {
	svc := ffmpeg.Get()
	if svc == nil {
		return ffmpeg.StreamInfo{}, ErrUnavailable
	}
	return svc.ProbeMediaStream(ctx, path)
}

func (e *MediaEngine) BuildPlan(ctx context.Context, realPath string, profile Profile) (plan hlsPlan, err error) {
	info, err := e.ProbeFile(ctx, realPath)
	if err != nil {
		return plan, err
	}
	if !info.HasVideo && !info.HasAudio {
		return plan, ErrUnsupportedMedia
	}

	stat, statErr := os.Stat(realPath)
	if statErr != nil {
		return plan, statErr
	}

	pipeline := ffmpeg.HLSPipelineOptions{
		MaxHeight:           maxHeightForProfile(profile),
		ForceVideoTranscode: profileForcesVideoTranscode(profile) || profileRequiresBitrateTranscode(profile, info),
	}
	buildIn := ffmpeg.BuildHLSSegmentBuildInput(info, pipeline)
	defaults := ffmpeg.DefaultOnDemandHLSDefaults()

	svc := ffmpeg.Get()
	if svc == nil {
		return plan, ErrUnavailable
	}
	params, err := svc.BuildHLSSegmentParams(ctx, realPath, buildIn, true)
	if err != nil {
		return plan, err
	}

	plan.info = info
	plan.params = params
	plan.pipeline = pipeline
	plan.profileMode = string(profile)
	plan.maxHeight = maxHeightForProfile(profile)
	plan.avgBitrateKbps = avgBitrateForProfile(profile)
	plan.remux = params.Remux
	plan.videoCopy = params.VideoCopy
	plan.needsTranscode = ffmpeg.NeedsFullVideoTranscode(info, pipeline)

	if plan.needsTranscode || (!plan.remux && !plan.videoCopy) {
		plan.encodeProfile = encodeProfileForMode(info, profile)
		plan.decodeProfile = ffmpeg.HLSDecodeProfileForOnDemand(info)
	}

	var keyframes []float64
	var starts, durs []float64
	if plan.remux || plan.videoCopy {
		keyframes, _ = svc.ProbeVideoKeyframeTimes(ctx, realPath)
		keyframes = ffmpeg.SanitizeHLSKeyframes(keyframes, info.Duration)
		starts, durs = ffmpeg.BuildHLSSegmentTimeline(info.Duration, keyframes, defaults.SegmentDurationSec)
	} else {
		// Full transcode output uses ffmpeg's fixed GOP/segment grid; align fMP4 to that grid.
		starts, durs = ffmpeg.BuildHLSSegmentTimeline(info.Duration, nil, defaults.SegmentDurationSec)
	}
	plan.keyframes = keyframes
	plan.segmentStarts = starts
	plan.segmentDurations = durs
	plan.segmentSec = defaults.SegmentDurationSec

	cacheID := ffmpeg.HLSCacheIdentity{
		SourcePath:         realPath,
		FileSize:           stat.Size(),
		FileModTime:        stat.ModTime().UnixNano(),
		Profile:            plan.profileMode,
		MaxResolution:      plan.maxHeight,
		MaxBitrate:         plan.avgBitrateKbps,
		SegmentDurationSec: plan.segmentSec,
		Params:             params,
	}
	plan.cacheFingerprint = ffmpeg.HLSCacheFingerprint(cacheID)
	return plan, nil
}

func (e *MediaEngine) StartContinuous(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, startSec float64) (ContinuousJob, error) {
	svc := ffmpeg.Get()
	if svc == nil {
		return nil, ErrUnavailable
	}
	opts := ffmpeg.HLSContinuousOptions{
		Input: ffmpeg.InputSource{
			URL:        realPath,
			StreamType: ffmpeg.StreamFile,
		},
		OutputDir:        outputDir,
		StartIndex:       startIndex,
		StartSec:         startSec,
		SegmentDurations: plan.segmentDurations,
		SegmentSec:       plan.segmentSec,
		FreshPlaylist:    startIndex == 0 && startSec <= 0,
		Decode:           plan.decodeProfile,
		Profile:          plan.encodeProfile,
		MaxHeight:        plan.maxHeight,
		Remux:            plan.remux,
		VideoCopy:        plan.videoCopy,
		GOP:              plan.params.GOP,
		Pacing:           ffmpeg.HLSContinuousCacheFill,
	}
	return svc.StartHLSContinuous(ctx, opts)
}

type hlsPlan struct {
	info             ffmpeg.StreamInfo
	params           ffmpeg.HLSSegmentParams
	pipeline         ffmpeg.HLSPipelineOptions
	profileMode      string
	maxHeight        int
	avgBitrateKbps   int
	remux            bool
	videoCopy        bool
	needsTranscode   bool
	encodeProfile    encode.VideoProfile
	decodeProfile    ffmpeg.VideoDecodeProfile
	keyframes        []float64
	segmentStarts    []float64
	segmentDurations []float64
	segmentSec       float64
	cacheFingerprint string
}

// maxHeightForProfile returns the configured output height cap for a profile.
// Zero means "keep the source resolution".
func maxHeightForProfile(profile Profile) int {
	cfg, ok := settings.TranscodeProfileConfig(string(profile))
	if !ok {
		return 0
	}
	return cfg.MaxResolution
}

func avgBitrateForProfile(profile Profile) int {
	cfg, ok := settings.TranscodeProfileConfig(string(profile))
	if !ok {
		return 0
	}
	return cfg.AvgBitrateKbps
}

// profileForcesVideoTranscode reports whether the profile always re-encodes.
// Quality preserves the source whenever stream copy is possible; the smaller
// profiles always transcode so output is normalized h264 video plus AAC stereo
// audio at a bounded bitrate, regardless of how the source was encoded.
func profileForcesVideoTranscode(profile Profile) bool {
	return profile != ProfileQuality
}

// profileRequiresBitrateTranscode reports whether the profile pins a bitrate cap
// below the source. Stream copy would silently ignore the cap, so a full video
// re-encode is required to honor the profile.
func profileRequiresBitrateTranscode(profile Profile, info ffmpeg.StreamInfo) bool {
	cfg, ok := settings.TranscodeProfileConfig(string(profile))
	if !ok || cfg.AvgBitrateKbps <= 0 {
		return false
	}
	return info.VideoBitrate <= 0 || info.VideoBitrate/1000 > cfg.AvgBitrateKbps
}

func encodeProfileForMode(info ffmpeg.StreamInfo, profile Profile) encode.VideoProfile {
	cfg, _ := settings.TranscodeProfileConfig(string(profile))
	maxHeight := cfg.MaxResolution
	// The scale filter can only cap the height, never upscale, so a cap above the
	// source resolution must not be used to pick a higher bitrate rung.
	if maxHeight <= 0 || (info.Height > 0 && info.Height < maxHeight) {
		maxHeight = info.Height
	}
	base := ffmpeg.DefaultHLSVideoProfile(maxHeight)
	switch profile {
	case ProfileDataSaver:
		base.Quality = encode.PresetVeryfast
	case ProfileBalanced:
		base.Quality = encode.PresetFast
	default:
		base.Quality = encode.PresetMedium
	}
	base.Bitrate = transcodeBitrateConfig(cfg, info, maxHeight)
	return base
}

// transcodeBitrateConfig resolves the target bitrate: explicit config wins, then the
// source bitrate when no downscale is applied, then a resolution-derived default.
func transcodeBitrateConfig(cfg settings.TranscodeProfile, info ffmpeg.StreamInfo, maxHeight int) encode.BitrateConfig {
	if cfg.AvgBitrateKbps > 0 {
		kbps := cfg.AvgBitrateKbps
		// The cap is a ceiling: -b:v is a target average, so a source already
		// below it would be inflated to the profile value. Floor at the source.
		if src := info.VideoBitrate / 1000; src > 0 && src < kbps {
			kbps = src
		}
		return bitrateFromKbps(kbps)
	}
	if info.VideoBitrate > 0 && (maxHeight <= 0 || info.Height <= maxHeight) {
		return bitrateFromKbps(info.VideoBitrate / 1000)
	}
	return ffmpeg.DefaultHLSVideoProfile(maxHeight).Bitrate
}

func bitrateFromKbps(kbps int) encode.BitrateConfig {
	if kbps < 1 {
		kbps = 1
	}
	return encode.BitrateConfig{
		Target:  fmt.Sprintf("%dk", kbps),
		Min:     fmt.Sprintf("%dk", kbps/2),
		Max:     fmt.Sprintf("%dk", kbps*3/2),
		BufSize: fmt.Sprintf("%dk", kbps*2),
	}
}

func userTranscodeLimit(uMax int) int {
	if uMax < 1 {
		return 1
	}
	return uMax
}

// continuousSeekParams maps a client playhead to ffmpeg's segment grid.
// FFmpeg starts at the segment boundary; the player seeks to the exact playhead.
func continuousSeekParams(startSec, segmentSec float64) (startIndex int, ffmpegStartSec float64) {
	if startSec <= 0 || segmentSec <= 0 {
		return 0, 0
	}
	startIndex = int(startSec / segmentSec)
	ffmpegStartSec = float64(startIndex) * segmentSec
	return startIndex, ffmpegStartSec
}

func normalizeClientID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "default"
	}
	if len(id) > 128 {
		return id[:128]
	}
	return id
}
