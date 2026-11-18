//go:build !386 && !arm

package imagemeta

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// A damaged JPEG has a valid SOI but no readable header. Its byte length must not
// make it win over a valid, smaller preview.
func TestReadLargestJPEGPreviewPrefersDecodableCandidate(t *testing.T) {
	var valid bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		img.Set(x, 0, color.RGBA{R: 200, G: 40, B: 40, A: 255})
	}
	require.NoError(t, jpeg.Encode(&valid, img, nil))

	damaged := append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{0x00}, 8192)...)
	require.Greater(t, len(damaged), valid.Len(), "damaged candidate must be the byte-wise larger one")

	path := filepath.Join(t.TempDir(), "previews.bin")
	require.NoError(t, os.WriteFile(path, append(append([]byte{}, valid.Bytes()...), damaged...), 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	candidates := []previewCandidate{
		{offset: 0, length: uint32(valid.Len())},
		{offset: uint32(valid.Len()), length: uint32(len(damaged))},
	}
	got, err := readLargestJPEGPreview(f, candidates)
	require.NoError(t, err)
	require.Equal(t, valid.Bytes(), got)
}

// With only damaged candidates, one is still returned so the caller can decide.
func TestReadLargestJPEGPreviewFallsBackToDamaged(t *testing.T) {
	damaged := append([]byte{0xFF, 0xD8, 0xFF}, bytes.Repeat([]byte{0x00}, 512)...)
	path := filepath.Join(t.TempDir(), "damaged.bin")
	require.NoError(t, os.WriteFile(path, damaged, 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	got, err := readLargestJPEGPreview(f, []previewCandidate{{offset: 0, length: uint32(len(damaged))}})
	require.NoError(t, err)
	require.Equal(t, damaged, got)
}
