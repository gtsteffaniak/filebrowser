package settings

func CanConvertImage(ext string) bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	if !Env.FFmpegAvailable {
		return false
	}
	val := Config.Integrations.Media.Convert.ImagePreview[ImagePreviewType(ext)]
	if val == nil {
		return false // Extension not in the configured list
	}
	return *val
}

func CanConvertVideo(ext string) bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	if !Env.FFmpegAvailable {
		return false
	}
	val := Config.Integrations.Media.Convert.VideoPreview[VideoPreviewType(ext)]
	if val == nil {
		return false // Extension not in the configured list
	}
	return *val
}

// MediaEnabled reports whether ffmpeg initialized successfully. Read under the
// media lock so it is consistent while an admin toggles media settings.
func MediaEnabled() bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Env.FFmpegAvailable
}
