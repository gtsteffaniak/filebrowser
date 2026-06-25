package settings

func CanConvertImage(ext string) bool {
	if !MediaEnabled() {
		return false
	}
	val := Config.Integrations.Media.Convert.ImagePreview[ImagePreviewType(ext)]
	if val == nil {
		return false // Extension not in the configured list
	}
	return *val
}

func CanConvertVideo(ext string) bool {
	if !MediaEnabled() {
		return false
	}
	val := Config.Integrations.Media.Convert.VideoPreview[VideoPreviewType(ext)]
	if val == nil {
		return false // Extension not in the configured list
	}
	return *val
}

func MediaEnabled() bool {
	return Env.FFmpegAvailable
}

func TranscodeEnabled() bool {
	return MediaEnabled() && Config.Integrations.Media.Transcode.Enabled
}
