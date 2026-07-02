package settings

import "strings"

const (
	MediaPresetQuality   = "quality"
	MediaPresetBalanced  = "balanced"
	MediaPresetDataSaver = "datasaver"
)

const (
	defaultQualityMaxResolution   = 1080
	defaultBalancedMaxResolution  = 720
	defaultDataSaverMaxResolution = 480
	defaultDataSaverMaxBitrate    = 1200
)

func normalizeMediaPresets() {
	p := &Config.Integrations.Media.Presets
	normalizeMediaPreset(&p.Quality, defaultQualityMaxResolution, 0)
	normalizeMediaPreset(&p.Balanced, defaultBalancedMaxResolution, 0)
	normalizeMediaPreset(&p.DataSaver, defaultDataSaverMaxResolution, defaultDataSaverMaxBitrate)
}

func normalizeMediaPreset(p *MediaPresetConfig, defaultMaxResolution, defaultMaxBitrate int) {
	if p.MaxResolution <= 0 {
		p.MaxResolution = defaultMaxResolution
	}
	if defaultMaxBitrate > 0 && p.MaxBitrate <= 0 {
		p.MaxBitrate = defaultMaxBitrate
	}
}

func normalizeMediaPresetMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case MediaPresetBalanced:
		return MediaPresetBalanced
	case MediaPresetDataSaver, "data-saver", "data_saver":
		return MediaPresetDataSaver
	default:
		return MediaPresetQuality
	}
}

// MediaPreset returns resolved settings for a profile name.
func MediaPreset(mode string) MediaPresetConfig {
	switch normalizeMediaPresetMode(mode) {
	case MediaPresetBalanced:
		return Config.Integrations.Media.Presets.Balanced
	case MediaPresetDataSaver:
		return Config.Integrations.Media.Presets.DataSaver
	default:
		return Config.Integrations.Media.Presets.Quality
	}
}

// MediaPresetMaxResolution returns the max output height in pixels for a profile.
func MediaPresetMaxResolution(mode string) int {
	p := MediaPreset(mode)
	if p.MaxResolution > 0 {
		return p.MaxResolution
	}
	switch normalizeMediaPresetMode(mode) {
	case MediaPresetBalanced:
		return defaultBalancedMaxResolution
	case MediaPresetDataSaver:
		return defaultDataSaverMaxResolution
	default:
		return defaultQualityMaxResolution
	}
}

// MediaPresetMaxBitrate returns configured max video bitrate in kbps (0 = automatic VBR).
func MediaPresetMaxBitrate(mode string) int {
	return MediaPreset(mode).MaxBitrate
}
