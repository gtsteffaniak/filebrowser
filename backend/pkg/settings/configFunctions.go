package settings

import "os"

func CanConvertImage(ext string) bool {
	imageType := ImagePreviewType(ext)

	val := Config.Integrations.Media.Convert.ImagePreview[imageType]
	if val == nil || !*val {
		return false
	}

	// HEIC preview uses libheif/heif-convert, not FFmpeg.
	if imageType == HEICImagePreview {
		_, err := os.Stat("/usr/bin/heif-convert")
		return err == nil
	}

	// Other image conversions still require FFmpeg.
	if !MediaEnabled() {
		return false
	}

	return true
}

func CanConvertVideo(ext string) bool {
	if !MediaEnabled() {
		return false
	}

	val := Config.Integrations.Media.Convert.VideoPreview[VideoPreviewType(ext)]
	if val == nil {
		return false
	}

	return *val
}

func MediaEnabled() bool {
	return Env.FFmpegAvailable
}
