package gomedia

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSupportsVideoContainer(t *testing.T) {
	cases := map[string]bool{
		"movie.mp4":      true,
		"MOVIE.MKV":      true,
		"clip.webm":      true,
		"clip.mov":       true,
		"clip.m4v":       true,
		"old.avi":        false,
		"legacy.wmv":     false,
		"stream.ts":      false,
		"no-extension":   false,
		"dir.mp4/folder": false,
	}
	for path, want := range cases {
		require.Equal(t, want, supportsVideoContainer(path), path)
	}
}

// TestNonTargetVideoContainerSkipsGoMedia checks that containers go-media does
// not target return the fallback result without reading the file. The path does
// not exist, so any file read would also fail. Only the extension gate can
// return false here before go-media is called.
func TestNonTargetVideoContainerSkipsGoMedia(t *testing.T) {
	withGate(t, false)
	missing := filepath.Join(t.TempDir(), "missing.wmv")

	_, ok := Probe(missing, nil)
	require.False(t, ok)
	_, ok = VideoThumbnail(missing, 256, 256, 72, 10, nil)
	require.False(t, ok)
	_, ok = EmbeddedSubtitles(missing, time.Now(), nil)
	require.False(t, ok)
	_, ok = ExtractSubtitle(missing, 0, nil)
	require.False(t, ok)
}

// encoderAvailable reports whether ffmpeg was built with the named encoder.
func encoderAvailable(t *testing.T, name string) bool {
	t.Helper()
	requireTool(t, "ffmpeg")
	out, err := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output()
	require.NoError(t, err)
	return bytes.Contains(out, []byte(" "+name+" "))
}

// makeMKV encodes a short clip with the given video encoder into an MKV file.
func makeMKV(t *testing.T, encoder string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip.mkv")
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25",
		"-t", "2", "-c:v", encoder, "-b:v", "500k", "-an", path).CombinedOutput()
	require.NoError(t, err, string(out))
	return path
}

// TestH264MatroskaThumbnailFallsBack checks that H.264 in MKV, which go-media
// does not decode, is reported as unsupported so the caller uses ffmpeg.
func TestH264MatroskaThumbnailFallsBack(t *testing.T) {
	withGate(t, false)
	if !encoderAvailable(t, "libx264") {
		t.Skip("ffmpeg has no libx264 encoder")
	}
	mkv := makeMKV(t, "libx264")

	_, ok := VideoThumbnail(mkv, 160, 120, 72, 10, nil)
	require.False(t, ok, "go-media must not thumbnail H.264 in MKV")
}

// TestVP8MatroskaThumbnailUsesGoMedia checks that the codec gate in go-media
// does not block VP8, which go-media still decodes.
func TestVP8MatroskaThumbnailUsesGoMedia(t *testing.T) {
	withGate(t, false)
	if !encoderAvailable(t, "libvpx") {
		t.Skip("ffmpeg has no libvpx encoder")
	}
	mkv := makeMKV(t, "libvpx")

	data, ok := VideoThumbnail(mkv, 160, 120, 72, 50, nil)
	require.True(t, ok, "go-media must thumbnail VP8 in MKV")
	require.True(t, strings.HasPrefix(string(data[:2]), "\xff\xd8"), "output must be JPEG")
}
