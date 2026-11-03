package preview

import (
	"bytes"
	"image"
	// Decoders for embedded images; the resize path needs them registered.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// useEmbeddedPreview reports whether the embedded image may be used for this file.
// With "Always use office for document previews" on, office files keep the office converter.
func useEmbeddedPreview(file iteminfo.ExtendedFileInfo) bool {
	if file.OnlyOfficeId != "" && settings.AlwaysUseOnlyOfficeEnabled() {
		return false
	}
	return true
}

// maxEmbeddedPixels caps width x height of an embedded image before any full decode.
// The read cap bounds encoded bytes, not decoded size: a small file can declare huge
// dimensions, and the resize path allocates every pixel (about 4 bytes each as RGBA).
const maxEmbeddedPixels = 40_000_000

// withinPixelBudget reports whether an image of the given size can be decoded safely.
func withinPixelBudget(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	return int64(width)*int64(height) <= maxEmbeddedPixels
}

// embeddedImageDecodes checks that embedded bytes are a real image whose declared size is
// within maxEmbeddedPixels, so a corrupt, stale, or oversized thumbnail falls through
// instead of failing or exhausting memory in the resize step. Only the header is read.
func embeddedImageDecodes(data []byte) bool {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return false
	}
	return withinPixelBudget(cfg.Width, cfg.Height)
}
