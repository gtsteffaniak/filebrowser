package settings

const (
	defaultTranscodeMaxResolution        = 1080
	defaultTranscodeDataSaverBitrateKbps = 900
)

func normalizeMediaTranscode() {
	t := &Config.Integrations.Media.Transcode
	if t.MaxResolution <= 0 {
		t.MaxResolution = defaultTranscodeMaxResolution
	}
	if t.DataSaverBitrateKbps <= 0 {
		t.DataSaverBitrateKbps = defaultTranscodeDataSaverBitrateKbps
	}
}

// TranscodeMaxResolution returns the max output height for quality transcode.
func TranscodeMaxResolution() int {
	n := Config.Integrations.Media.Transcode.MaxResolution
	if n <= 0 {
		return defaultTranscodeMaxResolution
	}
	return n
}

// TranscodeDataSaverBitrateKbps returns the max video bitrate cap for data saver at 720p output.
func TranscodeDataSaverBitrateKbps() int {
	n := Config.Integrations.Media.Transcode.DataSaverBitrateKbps
	if n <= 0 {
		return defaultTranscodeDataSaverBitrateKbps
	}
	return n
}
