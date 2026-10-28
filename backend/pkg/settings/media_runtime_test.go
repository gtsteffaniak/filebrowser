package settings

import (
	"sync"
	"testing"
	"time"
)

func withMediaConfig(t *testing.T) {
	t.Helper()
	original := Config.Integrations.Media
	originalInitialized := ffmpegInitialized
	originalAvailable := Env.FFmpegAvailable
	originalHWAvailable := Env.HardwareAccelerationAvailable
	t.Cleanup(func() {
		Config.Integrations.Media = original
		ffmpegInitialized = originalInitialized
		Env.FFmpegAvailable = originalAvailable
		Env.HardwareAccelerationAvailable = originalHWAvailable
	})
}

func TestMediaAccessorsReflectRuntimeSettings(t *testing.T) {
	withMediaConfig(t)
	originalTranscode := Config.Integrations.Media.Transcode
	t.Cleanup(func() { Config.Integrations.Media.Transcode = originalTranscode })

	ApplyMediaSettings(MediaSettings{
		Debug:                    true,
		ExtractEmbeddedSubtitles: true,
		HardwareAcceleration:     true,
		Transcode: MediaTranscode{
			Enabled:              true,
			MaxConcurrent:        4,
			MaxConcurrentPerUser: 2,
			CacheMaxSizeMB:       512,
			CacheRetentionH:      6,
			Profiles:             DefaultTranscodeProfiles(),
		},
	})

	if !MediaDebugEnabled() {
		t.Fatal("debug accessor did not reflect the applied setting")
	}
	if !ExtractEmbeddedSubtitlesEnabled() {
		t.Fatal("subtitle accessor did not reflect the applied setting")
	}
	if !HardwareAccelerationEnabled() {
		t.Fatal("hardware accessor did not reflect the applied setting")
	}
	if got := TranscodeMaxConcurrent(); got != 4 {
		t.Fatalf("TranscodeMaxConcurrent = %d, want 4", got)
	}
	if got := TranscodeMaxConcurrentPerUser(); got != 2 {
		t.Fatalf("TranscodeMaxConcurrentPerUser = %d, want 2", got)
	}
	if got := TranscodeCacheMaxSizeBytes(); got != 512*1024*1024 {
		t.Fatalf("TranscodeCacheMaxSizeBytes = %d", got)
	}
	if got := TranscodeCacheRetention(); got != 6*time.Hour {
		t.Fatalf("TranscodeCacheRetention = %v", got)
	}
	if got := EnabledTranscodeProfiles(); len(got) != 3 {
		t.Fatalf("EnabledTranscodeProfiles = %v, want 3", got)
	}
	snapshot := ConfigMediaIntegration()
	if !snapshot.HardwareAcceleration || !snapshot.Debug {
		t.Fatal("ConfigMediaIntegration snapshot is stale")
	}
}

func TestTranscodeAccessorsUseDefaultsWhenUnset(t *testing.T) {
	withMediaConfig(t)
	ApplyMediaSettings(MediaSettings{})

	if got := TranscodeMaxConcurrent(); got != 2 {
		t.Fatalf("TranscodeMaxConcurrent = %d, want the 2 default", got)
	}
	if got := TranscodeMaxConcurrentPerUser(); got != 1 {
		t.Fatalf("TranscodeMaxConcurrentPerUser = %d, want the 1 default", got)
	}
	if got := TranscodeCacheRetention(); got != 24*time.Hour {
		t.Fatalf("TranscodeCacheRetention = %v, want the 24h default", got)
	}
	// An unset cache size falls back to the shipped 10 GiB default rather than
	// leaving the cache unbounded.
	if got := TranscodeCacheMaxSizeBytes(); got != 10240*1024*1024 {
		t.Fatalf("TranscodeCacheMaxSizeBytes = %d, want the 10 GiB default", got)
	}
}

// SetDefaults ships a throttled producer rate of 2x realtime (never unthrottled).
func TestSetDefaultsTranscodeReadRate(t *testing.T) {
	defaults := SetDefaults(true)
	if got := defaults.Integrations.Media.Transcode.ReadRate; got != 2 {
		t.Fatalf("default readRate = %v, want 2", got)
	}
}

// The throttle handed to ffmpeg carries the rate plus a fixed initial burst so
// a seek fills the player buffer quickly; catch-up stays at ffmpeg's default.
func TestTranscodeThrottleConfigUsesRateAndInitialBurst(t *testing.T) {
	withMediaConfig(t)
	media := CurrentMediaSettings()
	media.Transcode.ReadRate = 1.5
	ApplyMediaSettings(media)

	cfg := TranscodeThrottleConfig()
	if !cfg.Enabled || cfg.Rate != 1.5 {
		t.Fatalf("throttle = %+v, want enabled at 1.5", cfg)
	}
	if cfg.InitialBurst != transcodeInitialBurstSec || cfg.Catchup != 0 {
		t.Fatalf("throttle = %+v, want initial burst and ffmpeg-default catchup", cfg)
	}
}

// Zero means unlimited once a size has been set explicitly.
func TestTranscodeCacheSizeZeroMeansUnlimited(t *testing.T) {
	withMediaConfig(t)
	media := CurrentMediaSettings()
	media.Transcode.Enabled = true
	media.Transcode.CacheMaxSizeMB = 0
	ApplyMediaSettings(media)
	if got := TranscodeCacheMaxSizeBytes(); got != 0 {
		t.Fatalf("TranscodeCacheMaxSizeBytes = %d, want 0 (unlimited) while transcoding is on", got)
	}
}

// CurrentMediaSettings must hand out an independent snapshot. The conversion maps
// hold *bool values, so a shallow copy would let a caller reconfigure the live
// server without ApplyMediaSettings, and would race with CanConvertImage.
func TestCurrentMediaSettingsReturnsIndependentConvertMaps(t *testing.T) {
	withMediaConfig(t)
	Config.Integrations.Media.Convert.ImagePreview = map[ImagePreviewType]*bool{
		HEICImagePreview: boolPtr(false),
		JPEGImagePreview: boolPtr(true),
		"nil-entry":      nil,
	}
	Config.Integrations.Media.Convert.VideoPreview = map[VideoPreviewType]*bool{
		MP4VideoPreview: boolPtr(true),
	}

	snapshot := CurrentMediaSettings()
	if snapshot.Convert.ImagePreview["nil-entry"] != nil {
		t.Fatal("a nil map entry must stay nil in the snapshot")
	}

	// Editing the snapshot in every way must leave the live config untouched.
	snapshot.Convert.ImagePreview[HEICImagePreview] = boolPtr(true)
	*snapshot.Convert.ImagePreview[JPEGImagePreview] = false
	snapshot.Convert.ImagePreview["added"] = boolPtr(true)
	delete(snapshot.Convert.ImagePreview, "nil-entry")
	snapshot.Convert.VideoPreview[MP4VideoPreview] = boolPtr(false)

	live := Config.Integrations.Media.Convert
	if *live.ImagePreview[HEICImagePreview] {
		t.Fatal("replacing a snapshot entry changed the live config")
	}
	if !*live.ImagePreview[JPEGImagePreview] {
		t.Fatal("writing through a snapshot pointer changed the live config")
	}
	if _, ok := live.ImagePreview["added"]; ok {
		t.Fatal("adding to the snapshot changed the live config")
	}
	if _, ok := live.ImagePreview["nil-entry"]; !ok {
		t.Fatal("deleting from the snapshot changed the live config")
	}
	if !*live.VideoPreview[MP4VideoPreview] {
		t.Fatal("the video preview map is still shared with the live config")
	}

	// Snapshots must not share pointers with each other or with the live config.
	fresh := CurrentMediaSettings()
	if fresh.Convert.ImagePreview[JPEGImagePreview] == snapshot.Convert.ImagePreview[JPEGImagePreview] {
		t.Fatal("two snapshots share a *bool pointer")
	}
	if fresh.Convert.ImagePreview[JPEGImagePreview] == live.ImagePreview[JPEGImagePreview] {
		t.Fatal("a snapshot shares a *bool pointer with the live config")
	}
	if !*fresh.Convert.ImagePreview[JPEGImagePreview] {
		t.Fatal("the live config should still report jpeg conversion enabled")
	}
}

// Detection runs once at startup: applying media settings (even init-relevant
// ones like hardware acceleration or the accel device) must never re-run it.
func TestApplyMediaSettingsNeverReinitializesFFmpeg(t *testing.T) {
	withMediaConfig(t)
	ffmpegInitialized = false

	media := CurrentMediaSettings()
	media.HardwareAcceleration = !media.HardwareAcceleration
	media.AccelDevice = "software"
	media.Debug = !media.Debug
	ApplyMediaSettings(media)

	if ffmpegInitialized {
		t.Fatal("ApplyMediaSettings must not initialize ffmpeg")
	}
	if got := CurrentMediaSettings(); got.AccelDevice != "software" || got.HardwareAcceleration != media.HardwareAcceleration {
		t.Fatalf("settings were not applied: %+v", got)
	}

	// mediaMu must be released after apply.
	acquired := make(chan bool)
	go func() { acquired <- HardwareAccelerationEnabled() }()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("mediaMu is still held after ApplyMediaSettings returned")
	}
}

// Media settings are read from request handlers and the transcode manager while
// an admin PATCH swaps them, so the accessors must be safe under concurrency.
func TestMediaAccessorsAreRaceFree(t *testing.T) {
	withMediaConfig(t)
	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		enabled := true
		for {
			select {
			case <-stop:
				return
			default:
			}
			enabled = !enabled
			ApplyMediaSettings(MediaSettings{
				Debug:                enabled,
				HardwareAcceleration: enabled,
				Transcode: MediaTranscode{
					Enabled:       enabled,
					MaxConcurrent: 2,
					Profiles:      DefaultTranscodeProfiles(),
				},
			})
		}
	}()

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_ = TranscodeEnabled()
				_ = TranscodeMaxConcurrent()
				_ = EnabledTranscodeProfiles()
				_, _ = TranscodeProfileConfig("quality")
				_ = MediaDebugEnabled()
				_ = CanConvertImage("heic")
				_ = CanConvertVideo("mp4")
				_ = CurrentMediaSettings()
				_ = MediaEnabled()
				_ = HardwareAccelerationSupported()
				_ = ExtractEmbeddedSubtitlesEnabled()
				_ = HardwareAccelerationEnabled()
			}
		}()
	}

	time.Sleep(200 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// Config load must not initialize ffmpeg: startup defers that to InitFFmpeg so the
// hardware codec probe runs once, after persisted media settings are applied.
func TestSetupMediaDefersFFmpegInit(t *testing.T) {
	withMediaConfig(t)
	originalAvailable := Env.FFmpegAvailable
	t.Cleanup(func() { Env.FFmpegAvailable = originalAvailable })

	Config.Integrations.Media.HardwareAcceleration = true
	Env.FFmpegAvailable = false

	setupMedia()

	if Env.FFmpegAvailable {
		t.Fatal("setupMedia must not initialize ffmpeg")
	}
}

func TestInitFFmpegIsIdempotent(t *testing.T) {
	withMediaConfig(t)
	originalInitialized := ffmpegInitialized
	originalAvailable := Env.FFmpegAvailable
	t.Cleanup(func() {
		ffmpegInitialized = originalInitialized
		Env.FFmpegAvailable = originalAvailable
	})

	// Already initialized (e.g. ApplyMediaSettings ran first): leave it alone.
	ffmpegInitialized = true
	Env.FFmpegAvailable = true
	InitFFmpeg()
	if !Env.FFmpegAvailable {
		t.Fatal("InitFFmpeg must not re-run an existing initialization")
	}

	// Not initialized: the probe is skipped under `go test`, so nothing changes.
	ffmpegInitialized = false
	Env.FFmpegAvailable = false
	InitFFmpeg()
	if ffmpegInitialized || Env.FFmpegAvailable {
		t.Fatal("InitFFmpeg must not run the probe during tests")
	}
}
