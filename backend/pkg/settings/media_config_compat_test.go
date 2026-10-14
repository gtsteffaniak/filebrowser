package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestLegacyMediaTranscodeConfigStillLoads guards against breaking existing
// config.yaml files: the loader uses yaml.DisallowUnknownField, so every field
// that used to be accepted under integrations.media must keep parsing even when
// it is deprecated and managed at runtime instead.
func TestLegacyMediaTranscodeConfigStillLoads(t *testing.T) {
	original := Config
	t.Cleanup(func() { Config = original })

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
    hardwareAcceleration: true
    debug: true
    extractEmbeddedSubtitles: true
    convert:
      imagePreview:
        heic: true
      videoPreview:
        mp4: false
    transcode:
      enabled: true
      maxResolution: 720
      maxConcurrent: 3
      cacheMaxSizeMB: 512
      cacheRetentionH: 12
`, filepath.Join(dir, "cache"), filepath.Join(dir, "db.sqlite"))

	configFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := loadConfigWithDefaults(configFile, true); err != nil {
		t.Fatalf("legacy media config should still load: %v", err)
	}

	media := Config.Integrations.Media
	if !media.Transcode.Enabled {
		t.Fatal("transcode.enabled was not applied")
	}
	if media.Transcode.MaxConcurrent != 3 {
		t.Fatalf("maxConcurrent = %d, want 3", media.Transcode.MaxConcurrent)
	}
	if media.Transcode.CacheMaxSizeMB != 512 {
		t.Fatalf("cacheMaxSizeMB = %d, want 512", media.Transcode.CacheMaxSizeMB)
	}
	if !media.HardwareAcceleration || !media.Debug || !media.ExtractEmbeddedSubtitles {
		t.Fatal("media toggles were not applied")
	}
	if media.Convert.ImagePreview[HEICImagePreview] == nil || !*media.Convert.ImagePreview[HEICImagePreview] {
		t.Fatal("imagePreview.heic was not applied")
	}
	if media.Convert.VideoPreview[VideoPreviewType("mp4")] == nil || *media.Convert.VideoPreview[VideoPreviewType("mp4")] {
		t.Fatal("videoPreview.mp4 was not applied")
	}
}
