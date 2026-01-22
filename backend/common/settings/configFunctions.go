package settings

func CanConvertImage(ext string) bool {
	if !MediaEnabled() {
		return false
	}
	// Pointer is guaranteed to be non-nil after defaults are applied
	return *Config.Integrations.Media.Convert.ImagePreview[ImagePreviewType(ext)]
}

func CanConvertVideo(ext string) bool {
	if !MediaEnabled() {
		return false
	}
	// Pointer is guaranteed to be non-nil after defaults are applied
	return *Config.Integrations.Media.Convert.VideoPreview[VideoPreviewType(ext)]
}

func MediaEnabled() bool {
	return Env.FFmpegPath != "" && Env.FFprobePath != ""
}
