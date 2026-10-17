package settings

import "sync"

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

// ffmpegInitInputs captures the settings that change how ffmpeg is initialized.
// Re-initializing (and re-running the multi-second hardware probe) is only needed
// when one of these changes, so unrelated media edits stay instant.
type ffmpegInitInputs struct {
	ffmpegPath           string
	hardwareAcceleration bool
	accelDevice          string
	debug                bool
	imageProcessors      int
	maxEncode            int
}

func currentFFmpegInitInputs() ffmpegInitInputs {
	n := Config.Server.NumImageProcessors
	if n < 1 {
		n = 4
	}
	maxEncode := Config.Integrations.Media.Transcode.MaxConcurrent
	if maxEncode < 1 {
		maxEncode = 2
	}
	return ffmpegInitInputs{
		ffmpegPath:           Config.Integrations.Media.FfmpegPath,
		hardwareAcceleration: Config.Integrations.Media.HardwareAcceleration,
		accelDevice:          Config.Integrations.Media.AccelDevice,
		debug:                Config.Integrations.Media.Debug,
		imageProcessors:      n,
		maxEncode:            maxEncode,
	}
}
