package preview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestGenerateRawPreviewRawWithoutEmbeddedReturnsUnsupported(t *testing.T) {
	data := make([]byte, 256)
	copy(data[:16], []byte("FUJIFILMCCD-RAW "))

	dir := t.TempDir()
	path := filepath.Join(dir, "empty.raf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	svc := NewPreviewGenerator(1, dir)
	file := iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{
				Name:    "empty.raf",
				Size:    int64(len(data)),
				ModTime: time.Now(),
				Type:    "image/x-fuji-raf",
			},
		},
		RealPath: path,
	}

	_, err := svc.generateRawPreview(context.Background(), file, "small", "", 0, "hash")
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("generateRawPreview() err = %v, want ErrUnsupportedFormat", err)
	}
	if strings.Contains(err.Error(), "SOI") {
		t.Fatalf("generateRawPreview() should fail fast, got JPEG decode error: %v", err)
	}
}

func TestGenerateImagePreviewUnsupportedExtensionReturnsUnsupported(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clip.mov")
	// Must exceed maxSizeForOriginal (256KB) so we don't short-circuit to raw bytes.
	payload := make([]byte, 256*1024+1)
	copy(payload, []byte("not a video"))
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	svc := NewPreviewGenerator(1, dir)
	file := iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{
				Name:    "clip.mov",
				Size:    int64(len(payload)),
				ModTime: time.Now(),
				Type:    "video/quicktime",
			},
		},
		RealPath: path,
	}

	_, err := svc.generateImagePreview(context.Background(), file, "small")
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("generateImagePreview() err = %v, want ErrUnsupportedFormat", err)
	}
}

func TestGenerateImagePreviewUsesPNGFormat(t *testing.T) {
	dir := t.TempDir()
	svc := NewPreviewGenerator(1, dir)

	// 1x1 red PNG; pad past maxSizeForOriginal (256KB) so we exercise format selection.
	const maxSizeForOriginal = 256 * 1024
	pngBytes := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x03, 0x01, 0x01, 0x00, 0x18, 0xdd, 0x8d,
		0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
	payload := make([]byte, maxSizeForOriginal+1)
	copy(payload, pngBytes)
	path := filepath.Join(dir, "pixel.png")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}

	file := iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{
				Name:    "pixel.png",
				Size:    int64(len(payload)),
				ModTime: time.Now(),
				Type:    "image/png",
			},
		},
		RealPath: path,
	}

	got, err := svc.generateImagePreview(context.Background(), file, "small")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 8 || string(got[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("expected PNG signature, got %x", got[:min(8, len(got))])
	}
}
