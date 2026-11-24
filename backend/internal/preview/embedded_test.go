package preview

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"archive/zip"

	"github.com/gtsteffaniak/filebrowser/backend/internal/embeddedpreview"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedImageDecodes(t *testing.T) {
	src := newGrayJpeg(t, 64, 64)
	defer src.Close()
	jpg, err := io.ReadAll(src)
	require.NoError(t, err)
	require.True(t, embeddedImageDecodes(jpg))
	require.False(t, embeddedImageDecodes([]byte("not an image at all, just text")))
}

func TestUseEmbeddedPreview_ignoresNonOfficeFiles(t *testing.T) {
	var fi iteminfo.ExtendedFileInfo
	fi.Name = "report.docx"
	require.True(t, useEmbeddedPreview(fi))
}

// A CBZ cover from the registry must decode and resize through the normal image path.
func TestEmbeddedPreview_cbzCoverResizes(t *testing.T) {
	src := newGrayJpeg(t, 400, 300)
	defer src.Close()
	jpg, err := io.ReadAll(src)
	require.NoError(t, err)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("001.jpg")
	require.NoError(t, err)
	_, err = w.Write(jpg)
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	path := filepath.Join(t.TempDir(), "comic.cbz")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))

	embedded, err := embeddedpreview.Extract(context.Background(), "comic.cbz", "application/x-cbz", path)
	require.NoError(t, err)
	require.True(t, embeddedImageDecodes(embedded.Data))

	options, err := getPreviewOptions("small")
	require.NoError(t, err)
	svc := NewPreviewGenerator(2, t.TempDir())
	out, err := svc.CreatePreview(context.Background(), bytes.NewReader(embedded.Data), int64(len(embedded.Data)), options)
	require.NoError(t, err)
	require.NotEmpty(t, out)
}

// A tiny PNG whose IHDR declares 10000x10000 must be rejected from its header alone,
// before any pixel buffer is allocated.
func TestEmbeddedImageDecodes_rejectsOversizedDeclaredDimensions(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 1, 1))))
	data := buf.Bytes()

	// IHDR chunk: length(4) "IHDR"(4) width(4) height(4) ... crc(4). Chunk starts at offset 8.
	binary.BigEndian.PutUint32(data[16:20], 10000)
	binary.BigEndian.PutUint32(data[20:24], 10000)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))

	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 10000, cfg.Width)
	require.False(t, embeddedImageDecodes(data))
}

// Folio documents skip the outer permit, so acquiring for the embedded path must take one.
func TestAcquireEmbeddedPermit_takesPermitWhenNotHeld(t *testing.T) {
	svc := &Service{imageSem: make(chan struct{}, 1)}

	release, err := svc.acquireEmbeddedPermit(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, svc.imageSem, 1, "permit must be held until release")

	release()
	require.Len(t, svc.imageSem, 0, "release must return the permit")
}

// With the only permit taken elsewhere, acquiring without a held permit waits and honours cancellation.
func TestAcquireEmbeddedPermit_waitsForPermit(t *testing.T) {
	svc := &Service{imageSem: make(chan struct{}, 1)}
	svc.imageSem <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	release, err := svc.acquireEmbeddedPermit(ctx, false)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, release)
}

// A caller that already holds the permit must not take another, so it never deadlocks.
func TestAcquireEmbeddedPermit_doesNotRetakeHeldPermit(t *testing.T) {
	svc := &Service{imageSem: make(chan struct{}, 1)}
	svc.imageSem <- struct{}{}

	release, err := svc.acquireEmbeddedPermit(context.Background(), true)
	require.NoError(t, err)
	release()
	require.Len(t, svc.imageSem, 1, "a no-op release must leave the caller's permit alone")
}

// Validation followed by resizing runs under one permit: the permit stays held through the resize
// and is released only once the caller is done.
func TestAcquireEmbeddedPermit_heldThroughValidationAndResize(t *testing.T) {
	src := newGrayJpeg(t, 400, 300)
	defer src.Close()
	jpg, err := io.ReadAll(src)
	require.NoError(t, err)

	svc := NewPreviewGenerator(1, t.TempDir())
	svc.imageSem = make(chan struct{}, 1)

	release, err := svc.acquireEmbeddedPermit(context.Background(), false)
	require.NoError(t, err)
	require.True(t, embeddedImageDecodes(jpg), "validation step")
	require.Len(t, svc.imageSem, 1, "permit must still be held after validation")

	options, err := getPreviewOptions("small")
	require.NoError(t, err)
	out, err := svc.CreatePreview(context.Background(), bytes.NewReader(jpg), int64(len(jpg)), options)
	require.NoError(t, err)
	require.NotEmpty(t, out)
	require.Len(t, svc.imageSem, 1, "permit must still be held after resizing")

	release()
	require.Len(t, svc.imageSem, 0)
}
