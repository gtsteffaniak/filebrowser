package preview

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

func TestHasEmbeddedPreview(t *testing.T) {
	if !hasEmbeddedPreview("image/x-canon-cr2", "photo.cr2") {
		t.Fatal("expected raw CR2 to have embedded preview path")
	}
	if !hasEmbeddedPreview("image/heic", "photo.heic") {
		t.Fatal("expected HEIC to have embedded preview path")
	}
	if hasEmbeddedPreview("image/jpeg", "photo.jpg") {
		t.Fatal("expected JPEG to skip embedded preview path")
	}
}

func TestIsJPEGMarker(t *testing.T) {
	if !isJPEG([]byte{0xff, 0xd8, 0xff}) {
		t.Fatal("expected JPEG SOI marker")
	}
}

// jpegWithDeclaredSize returns a valid small JPEG whose SOF header declares the given size.
// Only the header is rewritten, so building it costs nothing even for a huge declared size.
func jpegWithDeclaredSize(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)), nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	sof := bytes.Index(data, []byte{0xff, 0xc0})
	if sof < 0 {
		t.Fatal("no SOF0 marker in test JPEG")
	}
	// marker(2) length(2) precision(1) height(2) width(2)
	data[sof+5], data[sof+6] = byte(height>>8), byte(height)
	data[sof+7], data[sof+8] = byte(width>>8), byte(width)
	return data
}

func TestEmbeddedPreviewWithinLimits(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		want          bool
	}{
		{"normal camera preview", 6000, 4000, true},
		{"100MP medium format preview", 11648, 8736, true},
		{"declared size far beyond any camera", 60000, 60000, false},
		{"just over the limit", 12248, 12248, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := embeddedPreviewWithinLimits(jpegWithDeclaredSize(t, tt.width, tt.height)); got != tt.want {
				t.Fatalf("embeddedPreviewWithinLimits(%dx%d) = %v, want %v", tt.width, tt.height, got, tt.want)
			}
		})
	}
}

func TestEmbeddedPreviewWithinLimitsRejectsAnUnreadableHeader(t *testing.T) {
	if embeddedPreviewWithinLimits([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00}) {
		t.Fatal("a header that can't be read can't be size-checked, so it is rejected")
	}
}
