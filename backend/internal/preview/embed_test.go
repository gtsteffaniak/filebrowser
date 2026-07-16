package preview

import "testing"

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
