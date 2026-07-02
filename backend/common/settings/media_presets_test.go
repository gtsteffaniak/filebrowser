package settings

import "testing"

func TestMediaPresetDefaults(t *testing.T) {
	prev := Config.Integrations.Media.Presets
	t.Cleanup(func() {
		Config.Integrations.Media.Presets = prev
	})

	Config.Integrations.Media.Presets = MediaPresets{}
	normalizeMediaPresets()

	if got := MediaPresetMaxResolution(MediaPresetQuality); got != defaultQualityMaxResolution {
		t.Fatalf("quality maxResolution = %d", got)
	}
	if got := MediaPresetMaxResolution(MediaPresetBalanced); got != defaultBalancedMaxResolution {
		t.Fatalf("balanced maxResolution = %d", got)
	}
	if got := MediaPresetMaxResolution(MediaPresetDataSaver); got != defaultDataSaverMaxResolution {
		t.Fatalf("datasaver maxResolution = %d", got)
	}
	if got := MediaPresetMaxBitrate(MediaPresetDataSaver); got != defaultDataSaverMaxBitrate {
		t.Fatalf("datasaver maxBitrate = %d", got)
	}
}

func TestMediaPresetOverrides(t *testing.T) {
	prev := Config.Integrations.Media.Presets
	t.Cleanup(func() {
		Config.Integrations.Media.Presets = prev
	})

	Config.Integrations.Media.Presets.DataSaver.MaxBitrate = 1200
	Config.Integrations.Media.Presets.Balanced.MaxResolution = 576
	if got := MediaPresetMaxBitrate(MediaPresetDataSaver); got != 1200 {
		t.Fatalf("datasaver maxBitrate = %d", got)
	}
	if got := MediaPresetMaxResolution(MediaPresetBalanced); got != 576 {
		t.Fatalf("balanced maxResolution = %d", got)
	}
}

func TestNormalizeMediaPresetMode(t *testing.T) {
	t.Parallel()
	if got := normalizeMediaPresetMode("data-saver"); got != MediaPresetDataSaver {
		t.Fatalf("got %q", got)
	}
	if got := normalizeMediaPresetMode(""); got != MediaPresetQuality {
		t.Fatalf("got %q", got)
	}
}
