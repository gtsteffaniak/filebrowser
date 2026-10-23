package transcoding

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"

	"github.com/gtsteffaniak/go-logger/logger"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// maxCopySegmentDurationSec caps remux/video-copy segment length; sparse keyframe
// indexes otherwise plan multi-minute #EXTINF values and MSE-hostile fMP4.
func maxCopySegmentDurationSec(segmentSec float64) float64 {
	cap := segmentSec * 3
	if cap < 12 {
		cap = 12
	}
	return cap
}

func vodMaxSegmentDuration(list ffmpeg.VODSegmentList) float64 {
	var max float64
	for _, d := range list.Durations {
		if d > max {
			max = d
		}
	}
	return max
}

// MediaEngine adapts the ffmpeg service for HLS transcoding.
type MediaEngine struct{}

// VODJob is a running ffmpeg HLS VOD encode.
type VODJob interface {
	Cancel()
	Wait() error
	// Running reports whether ffmpeg is still active (non-blocking).
	Running() bool
	// VTDecodeUnreliable reports sustained VideoToolbox hardware decode failures
	// in ffmpeg stderr — a signal to restart the job with software decode.
	VTDecodeUnreliable() bool
}

// Engine performs probe/plan/start operations for tests and production.
type Engine interface {
	Available() bool
	BuildPlan(ctx context.Context, realPath string, profile Profile) (hlsPlan, error)
	StartVODJob(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, session *ffmpeg.HLSVODSession) (VODJob, error)
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
		(pipeline.MaxHeight > 0 && info.Height > pipeline.MaxHeight)
	pipeline.ForceVideoTranscode = mustTranscode
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
	// No audio stream to carry — keep it out of the ffmpeg args and reflect it
	// in the cache fingerprint instead of relying on the optional -map probe.
	params.OmitAudio = !info.HasAudio

	plan.info = info
	plan.params = params
	plan.profileMode = string(profile)
	plan.maxHeight = maxHeightForProfile(profile)
	plan.avgBitrateKbps = avgBitrateForProfile(profile)
	plan.remux = params.Remux
	plan.videoCopy = params.VideoCopy
	plan.needsTranscode = mustTranscode || (!plan.remux && !plan.videoCopy)

	if plan.needsTranscode {
		plan.setTranscodeProfiles(ctx, svc, realPath, profile)
	}

	var keyframes []float64
	if plan.remux || plan.videoCopy {
		rawKeyframes, _ := svc.ProbeVideoKeyframeTimes(ctx, realPath)
		keyframes, _ = ffmpeg.PrepareHLSMediaTimeline(rawKeyframes, info.Duration, info.StartTime)
	}
	plan.segmentSec = defaults.SegmentDurationSec
	plan.buildTimeline(keyframes)
	if plan.mediaDurationSec <= 0 {
		return plan, ErrUnsupportedMedia
	}

	if (plan.remux || plan.videoCopy) && vodMaxSegmentDuration(plan.vodList) > maxCopySegmentDurationSec(plan.segmentSec) {
		logger.Infof(
			"transcode: longest keyframe segment %.1fs exceeds %.1fs cap on %s — forcing video transcode for HLS",
			vodMaxSegmentDuration(plan.vodList),
			maxCopySegmentDurationSec(plan.segmentSec),
			realPath,
		)
		pipeline.ForceVideoTranscode = true
		buildIn = ffmpeg.BuildHLSSegmentBuildInput(info, pipeline)
		params, err = svc.BuildHLSSegmentParams(ctx, realPath, buildIn, true)
		if err != nil {
			return plan, err
		}
		params.OmitAudio = !info.HasAudio
		plan.params = params
		plan.remux = params.Remux
		plan.videoCopy = params.VideoCopy
		plan.needsTranscode = true
		plan.setTranscodeProfiles(ctx, svc, realPath, profile)
		plan.buildTimeline(nil)
	}

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
	// unverifiable timelines, so the cache namespace is versioned.
	plan.cacheFingerprint = "vod-" + ffmpeg.HLSCacheFingerprint(cacheID)
	return plan, nil
}

// setTranscodeProfiles resolves the encode/decode profiles for a transcode plan.
func (plan *hlsPlan) setTranscodeProfiles(ctx context.Context, svc *ffmpeg.Service, realPath string, profile Profile) {
	plan.encodeProfile = encodeProfileForMode(plan.info, profile)
	plan.decodeProfile = ffmpeg.HLSDecodeProfileForOnDemand(plan.info)
	// Hardware encoder limit checks need output height AND frame rate;
	// OutputHeight comes from DefaultHLSVideoProfile, fps from the probe.
	if fps, ferr := svc.ProbeVideoFPS(ctx, realPath); ferr == nil && fps > 0 {
		plan.encodeProfile.FrameRate = fps
	}
}

// buildTimeline plans the VOD segment list, media duration and playlist body
// from the probed info and the (already prepared) keyframes; nil keyframes
// select the fixed transcode grid.
func (plan *hlsPlan) buildTimeline(keyframes []float64) {
	info := plan.info
	plan.keyframes = keyframes
	_, plan.mediaDurationSec = ffmpeg.PrepareHLSMediaTimeline(nil, info.Duration, info.StartTime)
	plan.vodList = ffmpeg.BuildVODSegmentList(info.Duration, plan.segmentSec, info.StartTime, keyframes)
	if n := plan.vodList.Count(); n > 0 {
		last := n - 1
		plan.mediaDurationSec = plan.vodList.Starts[last] + plan.vodList.Durations[last]
	}
	plan.playlistBody = ffmpeg.BuildVODPlaylist(plan.vodList, ffmpeg.DefaultVODPlaylistOptions())
}

func (e *MediaEngine) StartVODJob(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, session *ffmpeg.HLSVODSession) (VODJob, error) {
	svc := ffmpeg.Get()
	if svc == nil {
		return nil, ErrUnavailable
	}
	opts := ffmpeg.HLSVODJobOptions{
		Input: ffmpeg.InputSource{
			URL:        realPath,
			StreamType: ffmpeg.StreamFile,
		},
		OutputDir:              outputDir,
		StartIndex:             startIndex,
		StartSec:               vodPlaylistStartSec(plan, startIndex),
		SegmentSec:             plan.segmentSec,
		InputStartTimeSec:      plan.info.StartTime,
		KeyframeSeekTimes:      plan.keyframes,
		Decode:                 plan.decodeProfile,
		Profile:                plan.encodeProfile,
		MaxHeight:              plan.maxHeight,
		Remux:                  plan.remux,
		VideoCopy:              plan.videoCopy,
		GOP:                    plan.params.GOP,
		InputVideoCodec:        plan.info.VideoCodec,
		OmitAudio:              plan.params.OmitAudio,
		Throttle:               settings.TranscodeThrottleConfig(),
		ClearSegmentsFromIndex: plan.vodClearFromIndex,
		Session:                session,
	}
	if planUsesExplicitCopy(plan) {
		// Explicit-cut copy jobs align restarts to the synthetic playlist grid
		// via -f segment -segment_times and the live .ffmpeg-vod-segments.csv.
		opts.SegmentList = &plan.vodList
	}
	job, err := svc.StartHLSVODSegmentJob(ctx, opts)
	if err != nil {
		return nil, err
	}
	return job, nil
}

type hlsPlan struct {
	info              ffmpeg.StreamInfo
	params            ffmpeg.HLSSegmentParams
	profileMode       string
	maxHeight         int
	avgBitrateKbps    int
	remux             bool
	videoCopy         bool
	needsTranscode    bool
	encodeProfile     encode.VideoProfile
	decodeProfile     ffmpeg.VideoDecodeProfile
	keyframes         []float64
	segmentSec        float64
	mediaDurationSec  float64
	cacheFingerprint  string
	vodList           ffmpeg.VODSegmentList
	playlistBody      string
	vodClearFromIndex bool
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
