package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func withConfigMediaPaths(t *testing.T, paths ...string) {
	t.Helper()
	original := Env.ConfigMediaSpecifiedPaths
	t.Cleanup(func() { Env.ConfigMediaSpecifiedPaths = original })
	Env.ConfigMediaSpecifiedPaths = paths
}

func TestMediaConfigLockPaths(t *testing.T) {
	withConfigMediaPaths(t, "hardwareAcceleration", "transcode.enabled")

	if !MediaSettingsLockedFromConfig() {
		t.Fatal("expected media settings to be reported as locked")
	}
	if !IsMediaSettingLockedFromConfig("hardwareAcceleration") {
		t.Fatal("hardwareAcceleration should be locked")
	}
	if IsMediaSettingLockedFromConfig("debug") {
		t.Fatal("debug was not set in config and must not be locked")
	}
	if got := ConfigSpecifiedMediaPaths(); len(got) != 2 {
		t.Fatalf("ConfigSpecifiedMediaPaths() = %v, want 2 entries", got)
	}
}

func TestValidateMediaPatchNotConfigLocked(t *testing.T) {
	withConfigMediaPaths(t, "hardwareAcceleration", "transcode.enabled")

	if err := ValidateMediaPatchNotConfigLocked([]byte(`{"debug":true}`)); err != nil {
		t.Fatalf("unlocked patch was rejected: %v", err)
	}
	if err := ValidateMediaPatchNotConfigLocked([]byte(`{"hardwareAcceleration":true}`)); err == nil {
		t.Fatal("patch touching a config-locked path must be rejected")
	}
	if err := ValidateMediaPatchNotConfigLocked([]byte(`{"transcode":{"enabled":true}}`)); err == nil {
		t.Fatal("nested config-locked path must be rejected")
	}
	if err := ValidateMediaPatchNotConfigLocked([]byte(`{"transcode":{"maxConcurrent":4}}`)); err != nil {
		t.Fatalf("sibling of a locked path must be allowed: %v", err)
	}
}

func TestValidateMediaPatchNotConfigLockedWithoutLocks(t *testing.T) {
	withConfigMediaPaths(t)

	if err := ValidateMediaPatchNotConfigLocked([]byte(`{"hardwareAcceleration":true}`)); err != nil {
		t.Fatalf("nothing is locked, patch should be accepted: %v", err)
	}
}

func TestApplyConfigSpecifiedPathsToMedia(t *testing.T) {
	withConfigMediaPaths(t, "hardwareAcceleration", "transcode.profiles.datasaver.avgBitrateKbps")

	stored := MediaSettings{
		Debug:                true,
		HardwareAcceleration: true,
		Transcode: MediaTranscode{
			Enabled: true,
			Profiles: TranscodeProfiles{
				DataSaver: TranscodeProfile{AvgBitrateKbps: 1500},
			},
		},
	}
	config := Media{
		HardwareAcceleration: false,
		Transcode: MediaTranscode{
			Profiles: TranscodeProfiles{
				DataSaver: TranscodeProfile{AvgBitrateKbps: 800},
			},
		},
	}

	overlaid, err := ApplyConfigSpecifiedPathsToMedia(stored, config)
	if err != nil {
		t.Fatalf("ApplyConfigSpecifiedPathsToMedia: %v", err)
	}
	if overlaid.HardwareAcceleration {
		t.Fatal("config hardwareAcceleration: false must win over the stored value")
	}
	if overlaid.Transcode.Profiles.DataSaver.AvgBitrateKbps != 800 {
		t.Fatalf("config bitrate must win, got %d", overlaid.Transcode.Profiles.DataSaver.AvgBitrateKbps)
	}
	if !overlaid.Debug || !overlaid.Transcode.Enabled {
		t.Fatal("fields not pinned in the config must keep their stored value")
	}
}

func TestApplyConfigSpecifiedPathsToMediaNoLocks(t *testing.T) {
	withConfigMediaPaths(t)

	stored := MediaSettings{HardwareAcceleration: true}
	overlaid, err := ApplyConfigSpecifiedPathsToMedia(stored, Media{HardwareAcceleration: false})
	if err != nil {
		t.Fatalf("ApplyConfigSpecifiedPathsToMedia: %v", err)
	}
	if !overlaid.HardwareAcceleration {
		t.Fatal("stored value must be untouched when nothing is pinned in config")
	}
}

// The loader records which integrations.media paths the config file pins, which
// drives both the runtime overlay and the hardware-detection opt-out.
func TestMediaConfigSpecifiedPathsCollectedFromConfig(t *testing.T) {
	originalConfig := Config
	t.Cleanup(func() { Config = originalConfig })
	withConfigMediaPaths(t)

	dir := t.TempDir()
	content := fmt.Sprintf(`server:
  cacheDir: %q
  database:
    path: %q
  sources:
    - path: "."
auth:
  key: "0123456789abcdef0123456789abcdef"
integrations:
  media:
    ffmpegPath: ""
    hardwareAcceleration: false
    convert:
      imagePreview:
        heic: true
    transcode:
      enabled: true
      maxConcurrent: 3
`, filepath.Join(dir, "cache"), filepath.Join(dir, "db.sqlite"))

	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := loadConfigWithDefaults(configFile, true); err != nil {
		t.Fatalf("load config: %v", err)
	}

	paths := ConfigSpecifiedMediaPaths()
	for _, want := range []string{
		"hardwareAcceleration",
		"convert.imagePreview.heic",
		"transcode.enabled",
		"transcode.maxConcurrent",
	} {
		if !slices.Contains(paths, want) {
			t.Fatalf("paths %v missing %q", paths, want)
		}
	}
	if slices.Contains(paths, "debug") {
		t.Fatalf("debug was not set in config but appears in %v", paths)
	}
	if !IsMediaSettingLockedFromConfig("hardwareAcceleration") {
		t.Fatal("hardwareAcceleration must be locked when pinned in the config file")
	}
}
