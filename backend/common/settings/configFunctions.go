package settings

func CanConvertImage(ext string) bool {
	return Config.Integrations.Media.FfmpegPath != "" && Config.Integrations.Media.Convert.ImagePreview[ImagePreviewType(ext)]
}

func CanConvertVideo(ext string) bool {
	return Config.Integrations.Media.FfmpegPath != "" && Config.Integrations.Media.Convert.VideoPreview[VideoPreviewType(ext)]
}
