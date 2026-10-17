package settings

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-logger/logger"
)

var ffmpegInitMu sync.Mutex

// HardwareAccelerationEnabled reports whether ffmpeg hardware acceleration is on.
func HardwareAccelerationEnabled() bool {
	return Config.Integrations.Media.HardwareAcceleration
}

// HardwareAccelerationSupported reports whether the host has a usable ffmpeg
// hardware encoder, as detected during initialization.
func HardwareAccelerationSupported() bool {
	return Env.HardwareAccelerationAvailable
}

// MediaDebugEnabled reports whether verbose ffmpeg output is enabled.
func MediaDebugEnabled() bool {
	return Config.Integrations.Media.Debug
}

// ExtractEmbeddedSubtitlesEnabled reports whether embedded subtitles are extracted.
func ExtractEmbeddedSubtitlesEnabled() bool {
	return Config.Integrations.Media.ExtractEmbeddedSubtitles
}

// TranscodeEnabled reports whether authenticated HLS transcoding is enabled.
func TranscodeEnabled() bool {
	return Config.Integrations.Media.Transcode.Enabled && Env.FFmpegAvailable
}

// TranscodeMaxConcurrent returns the global transcode session limit.
func TranscodeMaxConcurrent() int {
	n := Config.Integrations.Media.Transcode.MaxConcurrent
	if n < 1 {
		return 2
	}
	return n
}

// TranscodeMaxConcurrentPerUser returns the per-user concurrent transcode ceiling from config.
func TranscodeMaxConcurrentPerUser() int {
	n := Config.Integrations.Media.Transcode.MaxConcurrentPerUser
	if n < 1 {
		return 1
	}
	return n
}

// EnabledTranscodeProfiles returns enabled transcode preset ids in player-menu order.
func EnabledTranscodeProfiles() []string {
	p := Config.Integrations.Media.Transcode.Profiles
	ids := make([]string, 0, 3)
	for _, e := range []struct {
		id   string
		prof TranscodeProfile
	}{
		{"quality", p.Quality},
		{"balanced", p.Balanced},
		{"datasaver", p.DataSaver},
	} {
		if e.prof.Enabled {
			ids = append(ids, e.id)
		}
	}
	return ids
}

// TranscodeProfileConfig returns the configured settings for a transcode profile name.
func TranscodeProfileConfig(name string) (TranscodeProfile, bool) {
	p := Config.Integrations.Media.Transcode.Profiles
	switch name {
	case "quality":
		return p.Quality, true
	case "balanced":
		return p.Balanced, true
	case "datasaver":
		return p.DataSaver, true
	default:
		return TranscodeProfile{}, false
	}
}

// TranscodeEncoderBackend resolves the encoder backend for a transcode
// profile: the preset's own override when set, otherwise the transcode-level
// default, otherwise "" for the detected (auto) backend.
func TranscodeEncoderBackend(profile string) string {
	if cfg, ok := TranscodeProfileConfig(profile); ok && cfg.EncoderBackend != "" {
		return cfg.EncoderBackend
	}
	return Config.Integrations.Media.Transcode.EncoderBackend
}

// TranscodeCacheMaxSizeBytes returns the configured disk cache limit in bytes.
// Zero means unlimited.
func TranscodeCacheMaxSizeBytes() int64 {
	mb := Config.Integrations.Media.Transcode.CacheMaxSizeMB
	if mb <= 0 {
		return 0
	}
	return int64(mb) * 1024 * 1024
}

// TranscodeCacheRetention returns how long inactive cache generations are kept.
func TranscodeCacheRetention() time.Duration {
	h := Config.Integrations.Media.Transcode.CacheRetentionH
	if h <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(h) * time.Hour
}

func isGoTest() bool {
	return flag.Lookup("test.v") != nil
}

// InitFFmpeg probes ffmpeg and publishes Env. Startup calls it once after config
// load so the hardware codec probe does not run during YAML parsing.
func InitFFmpeg() {
	if isGoTest() {
		return
	}
	setupFFmpegIntegration()
}

func setupFFmpegIntegration() {
	ffmpegInitMu.Lock()
	defer ffmpegInitMu.Unlock()

	n := Config.Server.NumImageProcessors
	if n < 1 {
		n = 4
	}
	maxDecode := (n + 1) / 2
	maxEncode := Config.Integrations.Media.Transcode.MaxConcurrent
	if maxEncode < 1 {
		maxEncode = 2
	}
	initOpts := ffmpeg.InitOptions{
		FFmpegPath: Config.Integrations.Media.FfmpegPath,
		Concurrency: goffmpeg.Concurrency{
			MaxProbe:  16,
			MaxDecode: maxDecode,
			MaxEncode: maxEncode,
		},
		CacheDir:             Config.Server.CacheDir,
		HardwareAcceleration: Config.Integrations.Media.HardwareAcceleration,
		AccelDevice:          Config.Integrations.Media.AccelDevice,
		VerboseFFmpeg:        Config.Integrations.Media.Debug,
	}
	skipHwCheck := Config.Integrations.Media.SkipHwCheck
	if !Config.Integrations.Media.HardwareAcceleration || skipHwCheck {
		initOpts.EncoderHierarchy = []capabilities.AccelType{capabilities.AccelNone}
	}
	initOpts.SkipHWTests = skipHwCheck
	err := ffmpeg.Initialize(context.Background(), initOpts)
	if err != nil {
		logger.Warningf("ffmpeg unavailable: %v", err)
		publishFFmpegEnv(false, "", "", false)
		return
	}

	svc := ffmpeg.Get()
	if svc == nil {
		publishFFmpegEnv(false, "", "", false)
		return
	}

	hwAvailable := false
	if caps := svc.Capabilities(); caps != nil && len(caps.HardwareEncoders()) > 0 {
		hwAvailable = true
		logger.Debugf("ffmpeg capabilities: %s", caps.SummaryLine())
	}
	publishFFmpegEnv(true, svc.FFmpegPath(), svc.FFprobePath(), hwAvailable)
}

func publishFFmpegEnv(available bool, ffmpegPath, ffprobePath string, hardwareAcceleration bool) {
	Env.FFmpegAvailable = available
	Env.FFmpegPath = ffmpegPath
	Env.FFprobePath = ffprobePath
	Env.HardwareAccelerationAvailable = hardwareAcceleration
}
