//go:build !386 && !arm

package imagemeta

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRAFEmbeddedPreviewSynthetic(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01, 0xff, 0xd9}
	const jpegOffset = uint32(0x94)

	data := make([]byte, int(jpegOffset)+len(jpeg))
	copy(data[:rafMagicLen], []byte(rafMagic))
	binary.BigEndian.PutUint32(data[rafJPEGOffsetPos:rafJPEGLengthPos], jpegOffset)
	binary.BigEndian.PutUint32(data[rafJPEGLengthPos:rafHeaderMinBytes], uint32(len(jpeg)))
	copy(data[jpegOffset:], jpeg)

	path := filepath.Join(t.TempDir(), "sample.raf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ExtractEmbeddedPreview(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !IsJPEG(got) {
		t.Fatalf("expected JPEG preview, got %d bytes", len(got))
	}
	if len(got) != len(jpeg) {
		t.Fatalf("preview size = %d, want %d", len(got), len(jpeg))
	}
}

func TestExtractRAFEmbeddedPreviewIgnoresInvalidMagic(t *testing.T) {
	data := make([]byte, 256)
	copy(data[:8], []byte("NOTARAF "))
	path := filepath.Join(t.TempDir(), "bad.raf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ExtractEmbeddedPreview(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("ExtractEmbeddedPreview() = %v, want nil", got)
	}
}

func TestExtractRAFEmbeddedPreviewDSCF1276(t *testing.T) {
	path := "/Users/steffag/Downloads/DSCF1276.RAF"
	if _, err := os.Stat(path); err != nil {
		t.Skip(path)
	}

	got, err := ExtractEmbeddedPreview(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if !IsJPEG(got) {
		t.Fatalf("expected JPEG preview, got %d bytes", len(got))
	}
	if len(got) < 100 {
		t.Fatalf("preview size = %d, want >= 100", len(got))
	}
}

func TestExtractRAFEmbeddedPreviewRejectsOutOfBoundsOffset(t *testing.T) {
	jpeg := []byte{0xff, 0xd8, 0xff, 0xd9}
	data := make([]byte, 256)
	copy(data[:rafMagicLen], []byte(rafMagic))
	binary.BigEndian.PutUint32(data[rafJPEGOffsetPos:rafJPEGLengthPos], 200)
	binary.BigEndian.PutUint32(data[rafJPEGLengthPos:rafHeaderMinBytes], uint32(len(jpeg)))

	path := filepath.Join(t.TempDir(), "oob.raf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ExtractEmbeddedPreview(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("ExtractEmbeddedPreview() = %v, want nil", got)
	}
}
