package embeddedpreview

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

// pngBytes encodes a w x h image. Keep w x h small in tests.
func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))))
	return buf.Bytes()
}

// pngWithDeclaredSize rewrites the IHDR of a small PNG to declare w x h, with a valid CRC.
// The pixel data is still the small original, so the file is truncated relative to its header.
func pngWithDeclaredSize(t *testing.T, w, h uint32) []byte {
	t.Helper()
	data := pngBytes(t, 1, 1)
	binary.BigEndian.PutUint32(data[16:20], w)
	binary.BigEndian.PutUint32(data[20:24], h)
	binary.BigEndian.PutUint32(data[29:33], crc32.ChecksumIEEE(data[12:29]))
	return data
}

func TestWithinPixelBudget(t *testing.T) {
	require.True(t, WithinPixelBudget(1, 1))
	require.True(t, WithinPixelBudget(8000, 5000))
	require.False(t, WithinPixelBudget(0, 100))
	require.False(t, WithinPixelBudget(100, -1))
	require.False(t, WithinPixelBudget(10000, 10000))
}

func TestUsable_acceptsValidImage(t *testing.T) {
	require.True(t, Usable(pngBytes(t, 64, 64)))
}

func TestUsable_rejectsGarbage(t *testing.T) {
	require.False(t, Usable([]byte("not an image")))
}

func TestUsable_rejectsOversizedHeader(t *testing.T) {
	require.False(t, Usable(pngWithDeclaredSize(t, 10000, 10000)))
}

// The header parses and fits the budget, but the pixel data is cut short.
func TestUsable_rejectsTruncatedImage(t *testing.T) {
	full := pngBytes(t, 256, 256)
	truncated := full[:len(full)/2]
	_, _, err := image.DecodeConfig(bytes.NewReader(truncated))
	require.NoError(t, err, "header must still parse for this test to mean anything")
	require.False(t, Usable(truncated))
}

// The same PNG bytes the Playwright remote-image stub serves. They must be a real image.
func TestUsable_playwrightStubPNG(t *testing.T) {
	stub, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")
	require.NoError(t, err)
	require.True(t, Usable(stub))
}
