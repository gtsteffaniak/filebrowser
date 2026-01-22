package ffmpeg

import "github.com/gtsteffaniak/filebrowser/backend/common/settings"

func CanConvertImage(ext string) bool {
	if !MediaEnabled() {
		return false
	}
	val := settings.Config.Integrations.Media.Convert.ImagePreview[settings.ImagePreviewType(ext)]
	if val == nil {
		return false // Extension not in the configured list
	}
	return *val
}

func CanConvertVideo(ext string) bool {
	if !MediaEnabled() {
		return false
	}
	val := settings.Config.Integrations.Media.Convert.VideoPreview[settings.VideoPreviewType(ext)]
	if val == nil {
		return false // Extension not in the configured list
	}
	return *val
}

func MediaEnabled() bool {
	return settings.Env.FFmpegPath != "" && settings.Env.FFprobePath != ""
}
