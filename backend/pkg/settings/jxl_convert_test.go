package settings

import "testing"

// JPEG XL previews use go-media and must not depend on ffmpeg; HEIC still does.
func TestCanConvertImageJXLDoesNotRequireFFmpeg(t *testing.T) {
	withMediaConfig(t)
	Env.FFmpegAvailable = false

	original := Config.Integrations.Media.Convert.ImagePreview
	t.Cleanup(func() { Config.Integrations.Media.Convert.ImagePreview = original })
	Config.Integrations.Media.Convert.ImagePreview = map[ImagePreviewType]*bool{
		JXLImagePreview:  boolPtr(true),
		HEICImagePreview: boolPtr(true),
	}

	if !CanConvertImage(string(JXLImagePreview)) {
		t.Fatal("CanConvertImage(jxl) = false without ffmpeg; JPEG XL must not need ffmpeg")
	}
	if CanConvertImage(string(HEICImagePreview)) {
		t.Fatal("CanConvertImage(heic) = true without ffmpeg; HEIC must still require ffmpeg")
	}
}

func TestCanConvertImageJXLRespectsDisabledSetting(t *testing.T) {
	withMediaConfig(t)
	Env.FFmpegAvailable = false

	original := Config.Integrations.Media.Convert.ImagePreview
	t.Cleanup(func() { Config.Integrations.Media.Convert.ImagePreview = original })
	Config.Integrations.Media.Convert.ImagePreview = map[ImagePreviewType]*bool{
		JXLImagePreview: boolPtr(false),
	}

	if CanConvertImage(string(JXLImagePreview)) {
		t.Fatal("CanConvertImage(jxl) = true although jxl preview is disabled in config")
	}
}
