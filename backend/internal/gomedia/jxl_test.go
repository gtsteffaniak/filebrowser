package gomedia

import (
	"bytes"
	"errors"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/go-media/pkg/media"
	"github.com/stretchr/testify/require"
)

const fixtureJXL = "testdata/zoltan_unsplash.jxl"

// assertJPEG checks that data is a decodable JPEG.
func assertJPEG(t *testing.T, data []byte) {
	t.Helper()
	require.NotEmpty(t, data)
	_, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err, "go-media JXL output must be a valid JPEG")
}

func TestJXLJPEGDecodesFixture(t *testing.T) {
	withGate(t, false)
	data, ok := JXLJPEG(fixtureJXL, 85, nil)
	require.True(t, ok, "go-media must decode the JPEG XL fixture")
	assertJPEG(t, data)
}

// JPEG XL is decoded only by go-media, so the admin "Always use ffmpeg" setting
// must not stop it. ffmpeg cannot decode JPEG XL at all.
func TestJXLJPEGIgnoresAlwaysUseFfmpeg(t *testing.T) {
	withGate(t, true)
	data, ok := JXLJPEG(fixtureJXL, 85, nil)
	require.True(t, ok, "JPEG XL decode must not be disabled by the ffmpeg gate")
	assertJPEG(t, data)
}

// A symlink such as photo.jxl can resolve to a JPEG XL file with another name.
// Decoding must follow the content, not the resolved file's extension.
func TestJXLJPEGDecodesWithoutJXLExtension(t *testing.T) {
	withGate(t, false)
	data, err := os.ReadFile(fixtureJXL)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "photo.bin")
	require.NoError(t, os.WriteFile(path, data, 0o600))

	out, ok := JXLJPEG(path, 85, nil)
	require.True(t, ok, "JPEG XL content must decode regardless of the file extension")
	assertJPEG(t, out)
}

func TestJXLJPEGInvalidFileReturnsFalse(t *testing.T) {
	withGate(t, false)
	path := filepath.Join(t.TempDir(), "broken.jxl")
	require.NoError(t, os.WriteFile(path, []byte("not a jxl file"), 0o600))

	data, ok := JXLJPEG(path, 85, nil)
	require.False(t, ok, "a corrupt JPEG XL file must not decode")
	require.Empty(t, data)
}

func TestJXLJPEGMissingFileReturnsFalse(t *testing.T) {
	withGate(t, false)
	data, ok := JXLJPEG(filepath.Join(t.TempDir(), "missing.jxl"), 85, nil)
	require.False(t, ok)
	require.Empty(t, data)
}

// JPEG XL above stillImageMaxSourcePixels must fail with go-media's ErrUnsupported
// and return ok=false (no ffmpeg fallback).
func TestJXLJPEGRejectsOversizedSource(t *testing.T) {
	withGate(t, false)
	f, err := os.Open(fixtureJXL)
	require.NoError(t, err)
	defer f.Close()
	info, err := f.Stat()
	require.NoError(t, err)

	_, err = media.Thumbnail(f, info.Size(), media.ThumbnailOptions{
		MaxWidth:        256,
		MaxHeight:       256,
		Format:          "jpeg",
		Quality:         85,
		MaxSourcePixels: 1,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, media.ErrUnsupported), "oversized JXL must surface ErrUnsupported, got %v", err)
}
