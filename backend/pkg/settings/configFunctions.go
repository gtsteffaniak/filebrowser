package settings

func CanConvertImage(ext string) bool {
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
	if !Env.FFmpegAvailable {
		return false
	}
	val := Config.Integrations.Media.Convert.VideoPreview[VideoPreviewType(ext)]
	if val == nil {
		return false // Extension not in the configured list
	}
	return *val
}

// MediaEnabled reports whether ffmpeg initialized successfully.
func MediaEnabled() bool {
	return Env.FFmpegAvailable
}
