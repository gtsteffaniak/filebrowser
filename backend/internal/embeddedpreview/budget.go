package embeddedpreview

import (
	"bytes"
	"image"
	// Decoders for embedded images, so Usable can decode every format the preview path accepts.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// maxPixels caps width x height of an embedded image. The read cap bounds encoded bytes, not
// decoded size: a small file can declare huge dimensions, and decoding allocates every pixel
// (about 4 bytes each as RGBA).
const maxPixels = 40_000_000

// WithinPixelBudget reports whether an image of the given size can be decoded safely.
func WithinPixelBudget(width, height int) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	return int64(width)*int64(height) <= maxPixels
}

// Usable reports whether data is an image that is safe to use as a preview. The header must
// declare a size within the pixel budget, and the whole image must decode. The budget check runs
// first, so an oversized header is rejected without allocating pixels. The full decode catches
// truncated or corrupt files, which a header check alone accepts.
func Usable(data []byte) bool {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || !WithinPixelBudget(cfg.Width, cfg.Height) {
		return false
	}
	_, _, err = image.Decode(bytes.NewReader(data))
	return err == nil
}
