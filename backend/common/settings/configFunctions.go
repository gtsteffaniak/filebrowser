package settings

func CanConvertImage(ext string) bool {
	return MediaEnabled() && Config.Integrations.Media.Convert.ImagePreview[ImagePreviewType(ext)]
}

func CanConvertVideo(ext string) bool {
	return MediaEnabled() && Config.Integrations.Media.Convert.VideoPreview[VideoPreviewType(ext)]
}

func MediaEnabled() bool {
	return Env.FFmpegPath != "" && Env.FFprobePath != ""
}
