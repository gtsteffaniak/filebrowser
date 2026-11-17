package gomedia

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/go-media/pkg/media"
	"github.com/stretchr/testify/require"
)

// fixtureAVIFRotated has irot=1 (90° counter-clockwise) and imir axis 0.
const fixtureAVIFRotated = "testdata/irot1_imir0.avif"

// TestDisplayRotation checks the irot to ffmpeg display rotation mapping.
func TestDisplayRotation(t *testing.T) {
	cases := []struct {
		rotation int
		want     float64
	}{
		{0, 0}, {90, 90}, {180, 180}, {270, -90}, {-90, -90}, {360, 0}, {450, 90},
	}
	for _, c := range cases {
		require.Equal(t, c.want, displayRotation(media.ImageTransform{Rotation: c.rotation}), "rotation %d", c.rotation)
	}
	require.Equal(t, 90.0, displayRotation(media.ImageTransform{Rotation: 90, Mirror: true}), "a mirror does not change the rotation")
}

// TestHEICDisplayRotationReadsIrot checks the rotation read from a fixture
// with irot=1.
func TestHEICDisplayRotationReadsIrot(t *testing.T) {
	withGate(t, false)
	got, ok := HEICDisplayRotation(fixtureAVIFRotated)
	require.True(t, ok)
	require.Equal(t, 90.0, got)
}

// TestHEICDisplayRotationFallsBack checks that non-HEIF input, broken input,
// and the Always use ffmpeg setting leave the answer to ffmpeg.
func TestHEICDisplayRotationFallsBack(t *testing.T) {
	withGate(t, false)
	_, ok := HEICDisplayRotation(fixtureMP4)
	require.False(t, ok, "a non-HEIF file must fall back to the ffmpeg probe")

	junk := filepath.Join(t.TempDir(), "junk.heic")
	require.NoError(t, os.WriteFile(junk, []byte("not heif"), 0o644))
	_, ok = HEICDisplayRotation(junk)
	require.False(t, ok)

	withGate(t, true)
	_, ok = HEICDisplayRotation(fixtureAVIFRotated)
	require.False(t, ok, "the Always use ffmpeg setting must skip go-media")
}

// TestHEICDisplayRotationInstalled checks that loading this package installs
// go-media as the ffmpeg package's rotation reader.
func TestHEICDisplayRotationInstalled(t *testing.T) {
	withGate(t, false)
	var svc *ffmpeg.Service // nil: the ffmpeg probe cannot run, so a result proves the reader answered
	got, ok := svc.HEICDisplayRotation(t.Context(), fixtureAVIFRotated)
	require.True(t, ok)
	require.Equal(t, 90.0, got)
}

// TestHEICRotationDiff checks the shadow comparison of display rotations.
func TestHEICRotationDiff(t *testing.T) {
	require.Empty(t, heicRotationDiff(-90, -90))
	require.Empty(t, heicRotationDiff(180, -180))
	require.Empty(t, heicRotationDiff(0, math.Copysign(0, -1)))
	require.Len(t, heicRotationDiff(90, -90), 1)
}

// TestAVIFDecodesThroughHEICPath checks that an AVIF file is accepted by the
// HEIC decode path.
func TestAVIFDecodesThroughHEICPath(t *testing.T) {
	withGate(t, false)
	data, ok := HEICJPEG(fixtureAVIFRotated, 80, nil)
	require.True(t, ok)
	require.NotEmpty(t, data)
}
