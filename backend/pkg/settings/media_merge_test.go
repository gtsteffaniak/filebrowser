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
	if merged.Transcode.Profiles.Balanced.MaxResolution != 720 {
		t.Fatalf("balanced maxResolution = %d, want 720", merged.Transcode.Profiles.Balanced.MaxResolution)
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
	// An unset read rate is raised to the throttled default, not left off.
	if out.Transcode.ReadRate != 2 {
		t.Fatalf("readRate = %v, want the 2 default", out.Transcode.ReadRate)
	}
}

func TestDefaultTranscodeProfiles(t *testing.T) {
	// The capped tiers must not inherit the source bitrate ("0 = original").
	b := DefaultTranscodeProfiles().Balanced
	if b.MaxResolution != 720 || b.AvgBitrateKbps != 4000 {
		t.Fatalf("balanced = %dp/%d kbps, want 720p/4000 kbps", b.MaxResolution, b.AvgBitrateKbps)
	}
	ds := DefaultTranscodeProfiles().DataSaver
	if ds.MaxResolution != 480 || ds.AvgBitrateKbps != 1500 {
		t.Fatalf("data-saver = %dp/%d kbps, want 480p/1500 kbps", ds.MaxResolution, ds.AvgBitrateKbps)
	}
}

func TestSanitizeMediaSettingsAccelDevice(t *testing.T) {
	cases := []struct {
		name       string
		device     string
		hwAccel    bool
		wantDevice string
		wantHW     bool
	}{
		{"empty device defaults to auto", "", true, "auto", true},
		{"software disables hardware", "software", true, "software", false},
		{"cpu alias maps to software", "cpu", true, "software", false},
		{"bool off forces software", "auto", false, "software", false},
		{"named device stays enabled", "renderD128", true, "renderD128", true},
		{"uppercase normalizes", "AUTO", true, "auto", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := SanitizeMediaSettings(MediaSettings{
				AccelDevice:          tc.device,
				HardwareAcceleration: tc.hwAccel,
				Transcode:            MediaTranscode{MaxConcurrent: 1, MaxConcurrentPerUser: 1, CacheRetentionH: 1},
			})
			if out.AccelDevice != tc.wantDevice || out.HardwareAcceleration != tc.wantHW {
				t.Fatalf("accelDevice = %q/%v, want %q/%v", out.AccelDevice, out.HardwareAcceleration, tc.wantDevice, tc.wantHW)
			}
		})
	}
}

func TestMergeMediaSettingsPatchJSONAccelDeviceExplicit(t *testing.T) {
	base := testMediaSettings()
	base.AccelDevice = "software"
	base.HardwareAcceleration = false

	// An explicit accelDevice patch is authoritative over the derived bool:
	// switching the device away from software must re-enable hardware accel.
	merged, err := MergeMediaSettingsPatchJSON(base, []byte(`{"accelDevice":"auto"}`))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	merged = SanitizeMediaSettings(merged)
	if merged.AccelDevice != "auto" || !merged.HardwareAcceleration {
		t.Fatalf("accelDevice = %q/%v, want auto/true", merged.AccelDevice, merged.HardwareAcceleration)
	}
}

func TestMergeMediaSettingsPatchJSONLegacyBoolOff(t *testing.T) {
	base := testMediaSettings()
	base.AccelDevice = "auto"
	base.HardwareAcceleration = true

	// A legacy client toggling hardwareAcceleration off without touching
	// accelDevice must still land on the software device.
	merged, err := MergeMediaSettingsPatchJSON(base, []byte(`{"hardwareAcceleration":false}`))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	merged = SanitizeMediaSettings(merged)
	if merged.AccelDevice != "software" || merged.HardwareAcceleration {
		t.Fatalf("accelDevice = %q/%v, want software/false", merged.AccelDevice, merged.HardwareAcceleration)
	}
}

func TestSanitizeEncoderBackend(t *testing.T) {
	cases := map[string]string{
		"":             "",
		"auto":         "",
		"cpu":          "cpu",
		"software":     "cpu",
		"nvenc":        "nvenc",
		"VAAPI":        "vaapi",
		"videotoolbox": "videotoolbox",
		"h264_vaapi":   "h264_vaapi",
		"not-a-real":   "",
	}
	for in, want := range cases {
		if got := sanitizeEncoderBackend(in); got != want {
			t.Errorf("sanitizeEncoderBackend(%q) = %q, want %q", in, got, want)
		}
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

func TestSetDefaultsEnablesAutoAcceleration(t *testing.T) {
	m := SetDefaults(false).Integrations.Media
	if !m.HardwareAcceleration || m.AccelDevice != "auto" {
		t.Fatalf("defaults = hw %v device %q, want true/auto so go-ffmpeg picks the device hierarchy", m.HardwareAcceleration, m.AccelDevice)
	}
}
