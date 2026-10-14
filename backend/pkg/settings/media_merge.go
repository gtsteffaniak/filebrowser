package settings

import (
	"encoding/json"
	"fmt"
)

// MergeMediaSettingsPatchJSON merges a partial JSON patch into base media settings.
func MergeMediaSettingsPatchJSON(base MediaSettings, patchJSON []byte) (MediaSettings, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return MediaSettings{}, err
	}
	mergedBytes, err := mergeUserDefaultsJSON(baseBytes, patchJSON)
	if err != nil {
		return MediaSettings{}, err
	}
	var merged MediaSettings
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return MediaSettings{}, fmt.Errorf("unmarshal merged media settings: %w", err)
	}
	return merged, nil
}

// MediaSettingsEqual reports whether two runtime media settings are equivalent.
func MediaSettingsEqual(a, b MediaSettings) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

// ApplyMediaSettingsIfChanged applies runtime media settings, re-initializing ffmpeg
// only when they actually differ (avoids a redundant ffmpeg init on startup).
func ApplyMediaSettingsIfChanged(s MediaSettings) {
	if MediaSettingsEqual(CurrentMediaSettings(), s) {
		return
	}
	ApplyMediaSettings(s)
}

// SanitizeMediaSettings clamps runtime media settings into supported ranges.
func SanitizeMediaSettings(m MediaSettings) MediaSettings {
	t := &m.Transcode
	if t.MaxConcurrent < 1 {
		t.MaxConcurrent = 2
	}
	if t.MaxConcurrentPerUser < 1 {
		t.MaxConcurrentPerUser = 1
	}
	if t.CacheMaxSizeMB < 0 {
		t.CacheMaxSizeMB = 0
	}
	if t.CacheRetentionH < 1 {
		t.CacheRetentionH = 24
	}
	sanitizeTranscodeProfile(&t.Profiles.Quality)
	sanitizeTranscodeProfile(&t.Profiles.Balanced)
	sanitizeTranscodeProfile(&t.Profiles.DataSaver)
	return m
}

func sanitizeTranscodeProfile(p *TranscodeProfile) {
	if p.MaxResolution < 0 {
		p.MaxResolution = 0
	}
	if p.MaxResolution > 4320 {
		p.MaxResolution = 4320
	}
	if p.AvgBitrateKbps < 0 {
		p.AvgBitrateKbps = 0
	}
	if p.AvgBitrateKbps > 200000 {
		p.AvgBitrateKbps = 200000
	}
}
