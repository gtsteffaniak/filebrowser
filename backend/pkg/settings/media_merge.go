package settings

import (
	"encoding/json"
	"fmt"
	"strings"
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
	// An explicit accelDevice wins over the legacy hardwareAcceleration bool in
	// the base settings, which would otherwise force the device back to software.
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(patchJSON, &patch); err == nil {
		if raw, ok := patch["accelDevice"]; ok {
			var dev string
			if err := json.Unmarshal(raw, &dev); err == nil {
				merged.HardwareAcceleration = accelDeviceWantsHardware(dev)
			}
		}
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

// CloneMediaSettings returns a deep copy of m — the conversion maps hold *bool
// values, so a shallow copy would alias the live config's reference state.
func CloneMediaSettings(m MediaSettings) MediaSettings {
	out := m
	out.Convert = FfmpegConvert{
		ImagePreview: cloneBoolMap(m.Convert.ImagePreview),
		VideoPreview: cloneBoolMap(m.Convert.VideoPreview),
	}
	return out
}

// cloneBoolMap copies a map of *bool values, keeping nil entries nil.
func cloneBoolMap[K comparable](src map[K]*bool) map[K]*bool {
	if src == nil {
		return nil
	}
	dst := make(map[K]*bool, len(src))
	for k, v := range src {
		dst[k] = cloneBoolPtr(v)
	}
	return dst
}

func cloneBoolPtr(v *bool) *bool {
	if v == nil {
		return nil
	}
	return boolPtr(*v)
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
	clampTranscodeReadRate(t)
	sanitizeTranscodeProfile(&t.Profiles.Quality)
	sanitizeTranscodeProfile(&t.Profiles.Balanced)
	sanitizeTranscodeProfile(&t.Profiles.DataSaver)
	m.AccelDevice, m.HardwareAcceleration = sanitizeAccelDevice(m.AccelDevice, m.HardwareAcceleration)
	t.EncoderBackend = sanitizeEncoderBackend(t.EncoderBackend)
	for _, p := range []*TranscodeProfile{&t.Profiles.Quality, &t.Profiles.Balanced, &t.Profiles.DataSaver} {
		p.EncoderBackend = sanitizeEncoderBackend(p.EncoderBackend)
	}
	return m
}

// sanitizeAccelDevice normalizes the acceleration device selector and keeps the
// legacy hardwareAcceleration bool in sync: "software" is the only value that
// maps to disabled, and a false bool (set by older clients or a config pin)
// always forces the software device.
func sanitizeAccelDevice(device string, hwAccel bool) (string, bool) {
	dev := strings.TrimSpace(device)
	// Aliases compare case-insensitively; a named device (render node, GPU name)
	// keeps its case because render-node paths are case-sensitive.
	switch strings.ToLower(dev) {
	case "", "auto":
		dev = "auto"
	case "cpu", "none", "software":
		dev = "software"
	}
	if dev == "software" {
		return dev, false
	}
	if !hwAccel {
		return "software", false
	}
	return dev, true
}

// accelDeviceWantsHardware reports whether a device selector value enables
// hardware acceleration; only the software aliases opt out.
func accelDeviceWantsHardware(device string) bool {
	switch strings.ToLower(strings.TrimSpace(device)) {
	case "cpu", "none", "software":
		return false
	}
	return true
}

// sanitizeEncoderBackend normalizes an encoder backend preference: "" selects
// the detected default, "cpu" forces software, anything else must be a known
// accel backend name — unknown values degrade to auto rather than breaking
// transcodes.
func sanitizeEncoderBackend(backend string) string {
	switch strings.ToLower(strings.TrimSpace(backend)) {
	case "", "auto":
		return ""
	case "cpu", "software", "none":
		return "cpu"
	case "nvenc", "amf", "qsv", "vaapi", "d3d12", "videotoolbox":
		return strings.ToLower(strings.TrimSpace(backend))
	default:
		if looksLikeEncoderName(backend) {
			return backend
		}
		return ""
	}
}

func looksLikeEncoderName(name string) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		return false
	}
	lower := strings.ToLower(n)
	if strings.HasPrefix(lower, "lib") {
		return true
	}
	return strings.Contains(n, "_")
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
