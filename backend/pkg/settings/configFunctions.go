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

// FFmpegEnabled reports whether ffmpeg initialized successfully. This gates only
// the ffmpeg fallback and ffmpeg-only features (HEIC, transcoding). go-media does
// not need it. Read under the media lock so it is consistent while an admin
// toggles media settings.
func FFmpegEnabled() bool {
	mediaMu.RLock()
	defer mediaMu.RUnlock()
	return Env.FFmpegAvailable
}
