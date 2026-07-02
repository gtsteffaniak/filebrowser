package settings

// TranscodeMaxResolution returns the quality preset max output height
func TranscodeMaxResolution() int {
	return MediaPresetMaxResolution(MediaPresetQuality)
}
