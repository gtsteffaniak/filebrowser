package gomedia

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const fixtureWMV2 = fixtureDir + "/file_example_WMV_480_1_2MB.wmv"

// TestContainerGates pins which extensions go-media handles for probes and
// for thumbnails.
func TestContainerGates(t *testing.T) {
	cases := []struct {
		name             string
		probe, thumbnail bool
	}{
		{"a.mp4", true, true},
		{"a.M4V", true, true},
		{"a.mov", true, true},
		{"a.mkv", true, true},
		{"a.webm", true, true},
		{"a.wmv", false, true},
		{"a.ASF", false, true},
		{"a.avi", false, false},
		{"a.ts", false, false},
		{"a.m2ts", false, false},
		{"a.mts", false, false},
		{"a.mpg", false, false},
		{"a.mpeg", false, false},
		{"a.vob", false, false},
		{"a.flv", false, false},
	}
	for _, c := range cases {
		require.Equal(t, c.probe, supportsVideoContainer(c.name), "probe gate for %s", c.name)
		require.Equal(t, c.thumbnail, supportsVideoThumbnail(c.name), "thumbnail gate for %s", c.name)
	}
	for _, name := range []string{"a.heic", "a.HEIF", "a.heics", "a.avif"} {
		require.True(t, supportsHEIFImage(name), name)
	}
	require.False(t, supportsHEIFImage("a.jpg"))
}

// TestWMV2FallsBackToFFmpeg checks that a WMV2 file is not probed by go-media
// and that its thumbnail falls back to ffmpeg (go-media has no WMV1/WMV2
// decoder).
func TestWMV2FallsBackToFFmpeg(t *testing.T) {
	withGate(t, false)
	_, ok := Probe(fixtureWMV2, nil)
	require.False(t, ok, "WMV probes stay on ffprobe")
	_, ok = VideoThumbnail(fixtureWMV2, 320, 240, 80, 10, nil)
	require.False(t, ok, "WMV2 thumbnails must fall back to ffmpeg")
}

// TestWMV1FallsBackToFFmpeg is TestWMV2FallsBackToFFmpeg for a WMV1 stream
// encoded on the fly.
func TestWMV1FallsBackToFFmpeg(t *testing.T) {
	withGate(t, false)
	requireTool(t, "ffmpeg")
	path := filepath.Join(t.TempDir(), "wmv1.wmv")
	out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=25:duration=2",
		"-c:v", "wmv1", path).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg cannot encode wmv1: %v: %s", err, out)
	}
	_, err = os.Stat(path)
	require.NoError(t, err)
	_, ok := VideoThumbnail(path, 160, 120, 80, 10, nil)
	require.False(t, ok, "WMV1 thumbnails must fall back to ffmpeg")
}
