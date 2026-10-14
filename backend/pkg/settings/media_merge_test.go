package settings

import "testing"

func testMediaSettings() MediaSettings {
	return MediaSettings{
		Debug:                true,
		HardwareAcceleration: true,
		Transcode: MediaTranscode{
			Enabled:              true,
			MaxConcurrent:        2,
			MaxConcurrentPerUser: 1,
			CacheMaxSizeMB:       10240,
			CacheRetentionH:      24,
			Profiles:             DefaultTranscodeProfiles(),
		},
	}
}

func TestMergeMediaSettingsPatchJSON(t *testing.T) {
	base := testMediaSettings()

	merged, err := MergeMediaSettingsPatchJSON(base, []byte(`{"transcode":{"maxConcurrent":4}}`))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if merged.Transcode.MaxConcurrent != 4 {
		t.Fatalf("maxConcurrent = %d, want 4", merged.Transcode.MaxConcurrent)
	}
	if !merged.Transcode.Enabled {
		t.Fatal("enabled flag was lost")
	}
	if !merged.Transcode.Profiles.Quality.Enabled {
		t.Fatal("profiles were lost")
	}
	if !merged.Debug || !merged.HardwareAcceleration {
		t.Fatal("unrelated fields were lost")
	}
}

func TestMergeMediaSettingsPatchJSONProfile(t *testing.T) {
	base := testMediaSettings()

	merged, err := MergeMediaSettingsPatchJSON(base, []byte(`{"transcode":{"profiles":{"datasaver":{"maxResolution":480}}}}`))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if merged.Transcode.Profiles.DataSaver.MaxResolution != 480 {
		t.Fatalf("datasaver maxResolution = %d, want 480", merged.Transcode.Profiles.DataSaver.MaxResolution)
	}
	if merged.Transcode.Profiles.Balanced.MaxResolution != 1080 {
		t.Fatalf("balanced maxResolution = %d, want 1080", merged.Transcode.Profiles.Balanced.MaxResolution)
	}
}

func TestSanitizeMediaSettings(t *testing.T) {
	in := MediaSettings{
		Transcode: MediaTranscode{
			MaxConcurrent:        -1,
			MaxConcurrentPerUser: 0,
			CacheRetentionH:      0,
			Profiles: TranscodeProfiles{
				Quality: TranscodeProfile{MaxResolution: -5, AvgBitrateKbps: -1},
			},
		},
	}

	out := SanitizeMediaSettings(in)
	if out.Transcode.MaxConcurrent != 2 {
		t.Fatalf("maxConcurrent = %d, want 2", out.Transcode.MaxConcurrent)
	}
	if out.Transcode.MaxConcurrentPerUser != 1 {
		t.Fatalf("maxConcurrentPerUser = %d, want 1", out.Transcode.MaxConcurrentPerUser)
	}
	if out.Transcode.CacheRetentionH != 24 {
		t.Fatalf("cacheRetentionH = %d, want 24", out.Transcode.CacheRetentionH)
	}
	if out.Transcode.Profiles.Quality.MaxResolution != 0 {
		t.Fatalf("quality maxResolution = %d, want 0", out.Transcode.Profiles.Quality.MaxResolution)
	}
	if out.Transcode.Profiles.Quality.AvgBitrateKbps != 0 {
		t.Fatalf("quality avgBitrateKbps = %d, want 0", out.Transcode.Profiles.Quality.AvgBitrateKbps)
	}
}

func TestDefaultDataSaverBitrate(t *testing.T) {
	// A 720p data saver must not inherit the source bitrate ("0 = original").
	ds := DefaultTranscodeProfiles().DataSaver
	if ds.MaxResolution != 720 {
		t.Fatalf("data-saver maxResolution = %d, want 720", ds.MaxResolution)
	}
	if ds.AvgBitrateKbps != 1500 {
		t.Fatalf("data-saver avgBitrateKbps = %d, want 1500", ds.AvgBitrateKbps)
	}
}

func TestMigrateLegacyMediaDefaults(t *testing.T) {
	legacy := testMediaSettings()
	legacy.Transcode.Profiles.DataSaver = TranscodeProfile{Enabled: true, MaxResolution: 720, AvgBitrateKbps: 0}

	migrated, changed := MigrateLegacyMediaDefaults(legacy)
	if !changed {
		t.Fatal("expected the legacy data-saver default to be migrated")
	}
	if migrated.Transcode.Profiles.DataSaver.AvgBitrateKbps != 1500 {
		t.Fatalf("avgBitrateKbps = %d, want 1500", migrated.Transcode.Profiles.DataSaver.AvgBitrateKbps)
	}
}

func TestMigrateLegacyMediaDefaultsLeavesExplicitValues(t *testing.T) {
	custom := testMediaSettings()
	custom.Transcode.Profiles.DataSaver = TranscodeProfile{Enabled: true, MaxResolution: 720, AvgBitrateKbps: 800}
	if _, changed := MigrateLegacyMediaDefaults(custom); changed {
		t.Fatal("an explicit data-saver bitrate must not be migrated")
	}

	current := testMediaSettings()
	if _, changed := MigrateLegacyMediaDefaults(current); changed {
		t.Fatal("current defaults must not be migrated")
	}
}

func TestEnabledTranscodeProfiles(t *testing.T) {
	original := Config.Integrations.Media.Transcode
	t.Cleanup(func() { Config.Integrations.Media.Transcode = original })

	Config.Integrations.Media.Transcode.Profiles = DefaultTranscodeProfiles()
	if got := EnabledTranscodeProfiles(); len(got) != 3 {
		t.Fatalf("enabled profiles = %v, want 3", got)
	}

	Config.Integrations.Media.Transcode.Profiles.Balanced.Enabled = false
	got := EnabledTranscodeProfiles()
	if len(got) != 2 || got[0] != "quality" || got[1] != "datasaver" {
		t.Fatalf("enabled profiles = %v, want [quality datasaver]", got)
	}
}
