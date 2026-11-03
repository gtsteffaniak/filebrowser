package preview

import (
	"bytes"
	"context"
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
