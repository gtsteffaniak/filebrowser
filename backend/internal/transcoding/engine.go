package transcoding

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
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
	// VTDecodeUnreliable reports sustained VideoToolbox hardware decode failures
	// in ffmpeg stderr — a signal to restart the job with software decode.
	VTDecodeUnreliable() bool
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
		MaxHeight: maxHeightForProfile(profile),
	}
	mustTranscode := profileForcesVideoTranscode(profile) ||
		profileRequiresBitrateTranscode(profile, info) ||
		(pipeline.MaxHeight > 0 && info.Height > pipeline.MaxHeight) ||
		!fmp4CopyableVideoCodec(info.VideoCodec)
	pipeline.ForceVideoTranscode = mustTranscode
	// BuildHLSSegmentBuildInput gates stream copy on h264 only; fMP4 also
	// carries HEVC and VP9, so eligibility is computed here instead.
	remux, videoCopy := fmp4CopyEligibility(info, mustTranscode)
	buildIn := ffmpeg.HLSSegmentBuildInput{
		Info:      info,
		MaxHeight: pipeline.MaxHeight,
		Remux:     remux,
		VideoCopy: videoCopy,
	}
	defaults := ffmpeg.DefaultOnDemandHLSDefaults()

	svc := ffmpeg.Get()
	if svc == nil {
		return plan, ErrUnavailable
	}
	params, err := svc.BuildHLSSegmentParams(ctx, realPath, buildIn, true)
	if err != nil {
		return plan, err
	}
	// No audio stream to carry — keep it out of the ffmpeg args and reflect it
	// in the cache fingerprint instead of relying on the optional -map probe.
	params.OmitAudio = !info.HasAudio

	plan.info = info
	plan.params = params
	plan.pipeline = pipeline
	plan.profileMode = string(profile)
	plan.maxHeight = maxHeightForProfile(profile)
	plan.avgBitrateKbps = avgBitrateForProfile(profile)
	plan.remux = params.Remux
	plan.videoCopy = params.VideoCopy
	plan.needsTranscode = mustTranscode

	if plan.needsTranscode || (!plan.remux && !plan.videoCopy) {
		plan.encodeProfile = encodeProfileForMode(info, profile)
		plan.decodeProfile = ffmpeg.HLSDecodeProfileForOnDemand(info)
	}

	var keyframes []float64
	if plan.remux || plan.videoCopy {
		rawKeyframes, _ := svc.ProbeVideoKeyframeTimes(ctx, realPath)
		keyframes, plan.mediaDurationSec = ffmpeg.PrepareHLSMediaTimeline(rawKeyframes, info.Duration, info.StartTime)
	} else {
		_, plan.mediaDurationSec = ffmpeg.PrepareHLSMediaTimeline(nil, info.Duration, info.StartTime)
	}
	plan.keyframes = keyframes
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
	// Generations produced before the publisher model serve corrupt or
	// unverifiable timelines, so the cache namespace is versioned — v3 entries
	// always carry the published index.m3u8/job.json layout.
	plan.cacheFingerprint = "v3-" + ffmpeg.HLSCacheFingerprint(cacheID)
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
		OutputDir:         outputDir,
		StartIndex:        startIndex,
		StartSec:          startSec,
		SegmentSec:        plan.segmentSec,
		FreshPlaylist:     startIndex == 0 && startSec <= 0,
		Decode:            plan.decodeProfile,
		Profile:           plan.encodeProfile,
		MaxHeight:         plan.maxHeight,
		Remux:             plan.remux,
		VideoCopy:         plan.videoCopy,
		InputVideoCodec:   plan.info.VideoCodec,
		InputStartTimeSec: plan.info.StartTime,
		OmitAudio:         plan.params.OmitAudio,
		GOP:               plan.params.GOP,
		// ~1.5x input pacing after a 60s headstart burst: the produced edge
		// stays ahead of playback while the cache still only fills for
		// content that is actually watched.
		Pacing: ffmpeg.HLSContinuousLivePaced,
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
	segmentSec       float64
	mediaDurationSec float64
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

// fmp4CopyableVideoCodec reports whether the video codec can be stream-copied
// into fMP4 HLS output and still decode in every hls.js-capable browser. Only
// h264 qualifies: VP9 has no Safari MSE support, HEVC support is inconsistent
// outside Apple hardware, and VP8 has no MP4 mapping. Everything else takes
// the transcode path, which always emits h264 + AAC.
func fmp4CopyableVideoCodec(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	switch n {
	case "", "h264", "avc", "avc1":
		return true
	}
	return strings.Contains(n, "h264")
}

// fmp4CopyEligibility selects the stream-copy pipeline for a source: remux when
// video and audio are both fMP4-compatible, videoCopy when only the audio needs
// transcoding. Nothing copies when a transcode is required.
func fmp4CopyEligibility(info ffmpeg.StreamInfo, mustTranscode bool) (remux, videoCopy bool) {
	if !info.HasVideo || mustTranscode || !fmp4CopyableVideoCodec(info.VideoCodec) {
		return false, false
	}
	if info.AudioCodec == "" || strings.EqualFold(info.AudioCodec, "aac") {
		return true, false
	}
	return false, true
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
	accel, encoder := preferredEncoderSelection(profile)
	base.Accel = accel
	base.Encoder = encoder
	return base
}

// preferredEncoderSelection resolves the configured encoder backend for a profile:
// preset override, then the transcode-level default, then auto. "cpu" forces
// software; legacy accel names (vaapi, nvenc, …) select that backend; an
// explicit ffmpeg encoder name pins the encoder. Unavailable choices degrade to
// auto instead of failing jobs.
func preferredEncoderSelection(profile Profile) (capabilities.AccelType, string) {
	backend := settings.TranscodeEncoderBackend(string(profile))
	switch backend {
	case "", "auto":
		return "", ""
	case "cpu":
		return capabilities.AccelNone, ""
	default:
		if accel, ok := legacyAccelBackend(backend); ok {
			caps := ffmpeg.Get().Capabilities()
			if caps == nil || !caps.CodecEncodeAvailable(capabilities.CodecH264, accel) {
				return "", ""
			}
			return accel, ""
		}
		caps := ffmpeg.Get().Capabilities()
		if caps == nil {
			return "", ""
		}
		for _, opt := range caps.EncodeOptions {
			if opt.Codec != capabilities.CodecH264 || !opt.Available || opt.Encoder != backend {
				continue
			}
			if opt.Accel == "" || opt.Accel == capabilities.AccelNone {
				return capabilities.AccelNone, backend
			}
			return opt.Accel, backend
		}
		return "", ""
	}
}

func legacyAccelBackend(backend string) (capabilities.AccelType, bool) {
	switch backend {
	case "nvenc", "amf", "qsv", "vaapi", "d3d12", "videotoolbox":
		return capabilities.AccelType(backend), true
	default:
		return "", false
	}
}

// preferredEncoderAccel is kept for tests that only assert the accel override.
func preferredEncoderAccel(profile Profile) capabilities.AccelType {
	accel, _ := preferredEncoderSelection(profile)
	return accel
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
