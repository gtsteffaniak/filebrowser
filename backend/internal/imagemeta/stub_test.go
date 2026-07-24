//go:build 386 || arm

package imagemeta

import (
	"context"
	"testing"
)

func TestIsJPEG(t *testing.T) {
	if !IsJPEG([]byte{0xff, 0xd8, 0xff, 0xe0}) {
		t.Fatal("expected JPEG SOI")
	}
	if IsJPEG([]byte{0x89, 0x50}) {
		t.Fatal("expected non-JPEG")
	}
}

func TestExtractEmbeddedPreviewUnavailable(t *testing.T) {
	data, err := ExtractEmbeddedPreview(context.Background(), "/any/file.cr2")
	if err != nil {
		t.Fatalf("ExtractEmbeddedPreview() err = %v", err)
	}
	if data != nil {
		t.Fatalf("ExtractEmbeddedPreview() = %v, want nil on 32-bit", data)
	}
}

func TestGetOrientationUnavailable(t *testing.T) {
	if got := GetOrientation(context.Background(), "/any/file.cr2"); got != "" {
		t.Fatalf("GetOrientation() = %q, want empty on 32-bit", got)
	}
}
