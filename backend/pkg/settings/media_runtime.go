package settings

import (
	"context"
	"flag"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/gtsteffaniak/go-logger/logger"
)

// mediaMu guards the runtime media integration settings and the Env fields that
// ffmpeg initialization publishes. ApplyMediaSettings replaces whole sub-objects
// (including the conversion maps) while request handlers and the transcode
// manager read them, so every access goes through this lock.
var mediaMu sync.RWMutex

// ffmpegInitMu serializes ffmpeg (re-)initialization. The hardware codec probe
// takes seconds, so it runs outside mediaMu and only one probe may be in flight.
var ffmpegInitMu sync.Mutex

// ffmpegInitialized records whether ffmpeg has been initialized. Startup defers
// initialization until persisted media settings have been applied, and this keeps
// it to a single hardware codec probe even when ApplyMediaSettings runs first.
// Guarded by mediaMu.
var ffmpegInitialized bool

// ConfigMediaIntegration returns a snapshot of the media integration settings
// for callers that need more than one field.
func ConfigMediaIntegration() Media {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Config.Integrations.Media
}

// HardwareAccelerationEnabled reports whether ffmpeg hardware acceleration is on.
func HardwareAccelerationEnabled() bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Config.Integrations.Media.HardwareAcceleration
}

// AlwaysUseFfmpegEnabled reports whether the admin has forced ffmpeg for all
// media work, which skips the go-media fast path entirely.
func AlwaysUseFfmpegEnabled() bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Config.Integrations.Media.AlwaysUseFfmpeg
}

// HardwareAccelerationSupported reports whether the host has a usable ffmpeg
// hardware encoder, as detected during initialization.
func HardwareAccelerationSupported() bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Env.HardwareAccelerationAvailable
}

// MediaDebugEnabled reports whether verbose ffmpeg output is enabled.
func MediaDebugEnabled() bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Config.Integrations.Media.Debug
}

// ExtractEmbeddedSubtitlesEnabled reports whether embedded subtitles are extracted.
func ExtractEmbeddedSubtitlesEnabled() bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Config.Integrations.Media.ExtractEmbeddedSubtitles
}

// TranscodeEnabled reports whether authenticated HLS transcoding is enabled.
func TranscodeEnabled() bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Config.Integrations.Media.Transcode.Enabled && Env.FFmpegAvailable
}

// TranscodeMaxConcurrent returns the global transcode session limit.
func TranscodeMaxConcurrent() int {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	n := Config.Integrations.Media.Transcode.MaxConcurrent
	if n < 1 {
		return 2
	}
	return n
}

// TranscodeMaxConcurrentPerUser returns the admin-defined per-user ceiling.
func TranscodeMaxConcurrentPerUser() int {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	n := Config.Integrations.Media.Transcode.MaxConcurrentPerUser
	if n < 1 {
		return 1
	}
	return n
}

// EnabledTranscodeProfiles returns enabled transcode preset ids in player-menu order.
func EnabledTranscodeProfiles() []string {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return enabledTranscodeProfilesLocked()
}

func enabledTranscodeProfilesLocked() []string {
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
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return transcodeProfileConfigLocked(name)
}

// TranscodeEncoderBackend resolves the encoder backend for a transcode
// profile: the preset's own override when set, otherwise the transcode-level
// default, otherwise "" for the detected (auto) backend.
func TranscodeEncoderBackend(profile string) string {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	if cfg, ok := transcodeProfileConfigLocked(profile); ok && cfg.EncoderBackend != "" {
		return cfg.EncoderBackend
	}
	return Config.Integrations.Media.Transcode.EncoderBackend
}

func transcodeProfileConfigLocked(name string) (TranscodeProfile, bool) {
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

// TranscodeCacheMaxSizeBytes returns the configured disk cache limit in bytes.
// Zero means unlimited.
func TranscodeCacheMaxSizeBytes() int64 {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	mb := Config.Integrations.Media.Transcode.CacheMaxSizeMB
	if mb <= 0 {
		return 0
	}
	return int64(mb) * 1024 * 1024
}

// transcodeInitialBurstSec is how much media a producer reads at full speed
// before pacing starts. After a seek the player has no buffer, and with only
// the base readrate every segment arrives just in time, so playback stalls.
const transcodeInitialBurstSec = 20

// TranscodeThrottleConfig returns the readrate pacing applied to VOD producers.
// Catch-up pacing is left at ffmpeg's default. Enabled only when
// readRate > 0; the configured rate is always >= 1.
func TranscodeThrottleConfig() encode.ThrottleConfig {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	t := Config.Integrations.Media.Transcode
	return encode.ThrottleConfig{
		Enabled:      t.ReadRate > 0,
		Rate:         t.ReadRate,
		InitialBurst: transcodeInitialBurstSec,
	}
}

// TranscodeCacheRetention returns how long inactive cache generations are kept.
func TranscodeCacheRetention() time.Duration {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	h := Config.Integrations.Media.Transcode.CacheRetentionH
	if h <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(h) * time.Hour
}

// CurrentMediaSettings returns the runtime media settings (excluding ffmpegPath).
func CurrentMediaSettings() MediaSettings {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return currentMediaSettingsLocked()
}

// currentMediaSettingsLocked snapshots the live media settings. Callers must hold
// mediaMu. The returned value is deep-copied so it cannot alias the live config.
func currentMediaSettingsLocked() MediaSettings {
	m := Config.Integrations.Media
	return CloneMediaSettings(MediaSettings{
		Convert:                  m.Convert,
		Debug:                    m.Debug,
		ExtractEmbeddedSubtitles: m.ExtractEmbeddedSubtitles,
		HardwareAcceleration:     m.HardwareAcceleration,
		AccelDevice:              m.AccelDevice,
		AlwaysUseFfmpeg:          m.AlwaysUseFfmpeg,
		Transcode:                m.Transcode,
	})
}

// ApplyMediaSettings applies runtime media settings to the in-memory config.
// It never re-runs ffmpeg detection: codec/hardware detection happens exactly
// once at startup (InitFFmpeg). hardwareAcceleration is read live per transcode;
// ffmpegPath, accelDevice, debug and concurrency apply on the next restart.
func ApplyMediaSettings(s MediaSettings) {
	mediaMu.Lock()
	defer mediaMu.Unlock()
	Config.Integrations.Media.Convert = s.Convert
	Config.Integrations.Media.Debug = s.Debug
	Config.Integrations.Media.ExtractEmbeddedSubtitles = s.ExtractEmbeddedSubtitles
	Config.Integrations.Media.HardwareAcceleration = s.HardwareAcceleration
	Config.Integrations.Media.AccelDevice = s.AccelDevice
	Config.Integrations.Media.AlwaysUseFfmpeg = s.AlwaysUseFfmpeg
	Config.Integrations.Media.Transcode = s.Transcode
	applyMediaConvertDefaults()
	setupMediaTranscode()
}

func isGoTest() bool {
	return flag.Lookup("test.v") != nil
}

// InitFFmpeg initializes ffmpeg with the effective media settings. Startup calls
// it once, after persisted media settings have been applied, so the multi-second
// hardware codec probe does not run twice. It is a no-op when ffmpeg has already
// been initialized.
func InitFFmpeg() {
	mediaMu.Lock()
	if isGoTest() || ffmpegInitialized {
		mediaMu.Unlock()
		return
	}
	ffmpegInitialized = true
	mediaMu.Unlock()
	setupFFmpegIntegration()
}

// setupFFmpegIntegration probes ffmpeg and publishes the resulting Env fields.
// The probe runs without mediaMu so media readers (transcode admission, preview
// type selection, static page rendering) are not blocked for seconds, and it is
// serialized by ffmpegInitMu. Callers must not hold mediaMu.
func setupFFmpegIntegration() {
	ffmpegInitMu.Lock()
	defer ffmpegInitMu.Unlock()

	mediaMu.RLock()
	numImageProcessors := Config.Server.NumImageProcessors
	cacheDir := Config.Server.CacheDir
	ffmpegPath := Config.Integrations.Media.FfmpegPath
	hardwareAcceleration := Config.Integrations.Media.HardwareAcceleration
	skipHwCheck := Config.Integrations.Media.SkipHwCheck
	accelDevice := Config.Integrations.Media.AccelDevice
	debug := Config.Integrations.Media.Debug
	maxConcurrent := Config.Integrations.Media.Transcode.MaxConcurrent
	mediaMu.RUnlock()

	n := numImageProcessors
	if n < 1 {
		n = 4
	}
	maxDecode := (n + 1) / 2
	maxEncode := maxConcurrent
	if maxEncode < 1 {
		maxEncode = 2
	}
	initOpts := ffmpeg.InitOptions{
		FFmpegPath: ffmpegPath,
		Concurrency: goffmpeg.Concurrency{
			MaxProbe:  16,
			MaxDecode: maxDecode,
			MaxEncode: maxEncode,
		},
		CacheDir:             cacheDir,
		HardwareAcceleration: hardwareAcceleration,
		AccelDevice:          accelDevice,
		VerboseFFmpeg:        debug,
	}
	// Hardware codec detection always runs once at startup so the
	// hardwareAcceleration setting can be toggled live; it is applied per
	// transcode, not by re-detecting. skipHwCheck (config-only) skips the probe
	// entirely — e.g. for CI startup.
	if skipHwCheck {
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

	publishFFmpegEnv(true, svc.FFmpegPath(), svc.FFprobePath(), svc.HardwareAccelerationAvailable())
}

// publishFFmpegEnv records the ffmpeg detection results under the media lock so
// readers never observe a partially updated set of Env fields.
func publishFFmpegEnv(available bool, ffmpegPath, ffprobePath string, hardwareAcceleration bool) {
	mediaMu.Lock()
	defer mediaMu.Unlock()
	Env.FFmpegAvailable = available
	Env.FFmpegPath = ffmpegPath
	Env.FFprobePath = ffprobePath
	Env.HardwareAccelerationAvailable = hardwareAcceleration
}
