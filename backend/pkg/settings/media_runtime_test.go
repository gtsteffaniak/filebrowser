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

// Only settings that change how ffmpeg is initialized should trigger a re-probe.
func TestFFmpegInitInputsTrackOnlyInitRelevantFields(t *testing.T) {
	withMediaConfig(t)
	Config.Integrations.Media.HardwareAcceleration = false
	Config.Integrations.Media.Debug = false
	Config.Integrations.Media.Transcode.MaxConcurrent = 2
	baseline := currentFFmpegInitInputs()

	// A conversion-format or profile change must not look like an init change.
	media := CurrentMediaSettings()
	media.Transcode.Profiles.DataSaver.AvgBitrateKbps = 900
	media.Transcode.CacheRetentionH = 12
	ApplyMediaSettings(media)
	if got := currentFFmpegInitInputs(); got != baseline {
		t.Fatalf("non-init settings changed the init inputs: %+v vs %+v", got, baseline)
	}

	// Enabling hardware acceleration does.
	media.HardwareAcceleration = true
	ApplyMediaSettings(media)
	if got := currentFFmpegInitInputs(); got == baseline {
		t.Fatal("enabling hardware acceleration must register as an init change")
	}

	// So does raising the encode concurrency.
	media2 := CurrentMediaSettings()
	media2.Transcode.MaxConcurrent = 6
	before := currentFFmpegInitInputs()
	ApplyMediaSettings(media2)
	if got := currentFFmpegInitInputs(); got == before {
		t.Fatal("changing transcode concurrency must register as an init change")
	}
}

// Changing an init-relevant input marks ffmpeg initialized (so a later startup
// probe is skipped) and returns with mediaMu released, so media readers are never
// blocked behind the multi-second hardware probe.
func TestApplyMediaSettingsReleasesLockAndMarksInitialized(t *testing.T) {
	withMediaConfig(t)
	ffmpegInitialized = false

	media := CurrentMediaSettings()
	media.HardwareAcceleration = !media.HardwareAcceleration
	ApplyMediaSettings(media)

	if !ffmpegInitialized {
		t.Fatal("an init-input change must mark ffmpeg as initialized")
	}

	// Reading through a locked accessor must not block: ApplyMediaSettings has
	// returned and released mediaMu.
	acquired := make(chan bool)
	go func() { acquired <- HardwareAccelerationEnabled() }()
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("mediaMu is still held after ApplyMediaSettings returned")
	}
}

// A patch that touches no init-relevant input must not trigger a re-probe.
func TestApplyMediaSettingsSkipsInitWhenNothingRelevantChanged(t *testing.T) {
	withMediaConfig(t)

	media := CurrentMediaSettings()
	media.Debug = !media.Debug // debug IS init-relevant
	ffmpegInitialized = false
	ApplyMediaSettings(media)
	if !ffmpegInitialized {
		t.Fatal("debug is an init input and must mark ffmpeg initialized")
	}

	ffmpegInitialized = false
	media = CurrentMediaSettings()
	media.Transcode.Profiles.DataSaver.AvgBitrateKbps = 1234
	media.Transcode.CacheRetentionH = 12
	ApplyMediaSettings(media)
	if ffmpegInitialized {
		t.Fatal("a profile/cache-only change must not trigger a re-probe")
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
