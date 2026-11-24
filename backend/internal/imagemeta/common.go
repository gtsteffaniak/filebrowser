package imagemeta

// MaxPreviewPixels is the largest embedded image, in pixels, that the preview path decodes.
// It lives in common.go (no build tag) so it is available on 32-bit platforms too, where
// embedded.go is excluded and embeddedpreview still references it.
const MaxPreviewPixels = 40_000_000

// IsJPEG reports whether data begins with a JPEG SOI marker.
func IsJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8
}
