package preview

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// withHEICConversion sets ffmpeg availability and the HEIC preview setting for
// one test.
func withHEICConversion(t *testing.T, enabled bool) {
	t.Helper()
	prevAvail := settings.Env.FFmpegAvailable
	prevPreview := settings.Config.Integrations.Media.Convert.ImagePreview
	t.Cleanup(func() {
		settings.Env.FFmpegAvailable = prevAvail
		settings.Config.Integrations.Media.Convert.ImagePreview = prevPreview
	})
	settings.Env.FFmpegAvailable = true
	settings.Config.Integrations.Media.Convert.ImagePreview = map[settings.ImagePreviewType]*bool{
		settings.HEICImagePreview: &enabled,
	}
}

// previewFile returns a file info with only a name and MIME type set.
func previewFile(name, mimeType string) iteminfo.ExtendedFileInfo {
	var f iteminfo.ExtendedFileInfo
	f.Name = name
	f.Type = mimeType
	return f
}

// TestDeterminePreviewTypeAVIF checks that AVIF uses the HEIF decode path when
// HEIC conversion is enabled, and the plain image path otherwise.
func TestDeterminePreviewTypeAVIF(t *testing.T) {
	withHEICConversion(t, true)
	if got := determinePreviewType(previewFile("a.avif", "image/avif")); got != previewTypeHEIC {
		t.Fatalf("avif with HEIC conversion = %v, want previewTypeHEIC", got)
	}
	if got := determinePreviewType(previewFile("a.heic", "image/heic")); got != previewTypeHEIC {
		t.Fatalf("heic with HEIC conversion = %v, want previewTypeHEIC", got)
	}

	withHEICConversion(t, false)
	if got := determinePreviewType(previewFile("a.avif", "image/avif")); got != previewTypeImage {
		t.Fatalf("avif without HEIC conversion = %v, want previewTypeImage", got)
	}
}
