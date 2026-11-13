package transcoding

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const keyframeFixtureMP4 = "../../../frontend/tests/playwright-files/myfolder/testdata/sample-640x480.mp4"

// TestProbeKeyframesCachedUsesGoMedia checks that an MP4 keyframe timeline is
// built without ffprobe: with no ffmpeg service, only go-media can answer.
func TestProbeKeyframesCachedUsesGoMedia(t *testing.T) {
	if settings.AlwaysUseFfmpegEnabled() {
		t.Skip("go-media disabled by settings")
	}
	prevCacheDir := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = "" // keep the disk cache out of the test
	t.Cleanup(func() { settings.Config.Server.CacheDir = prevCacheDir })

	path := filepath.Join(t.TempDir(), "sample.mp4")
	data, err := os.ReadFile(keyframeFixtureMP4)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	times, err := probeKeyframesCached(context.Background(), nil, path, st.Size(), st.ModTime().UnixNano(), 4, true)
	if err != nil {
		t.Fatalf("probeKeyframesCached: %v", err)
	}
	if len(times) == 0 || times[0] < 0 {
		t.Fatalf("unexpected keyframes %v", times)
	}
	for i := 1; i < len(times); i++ {
		if times[i] <= times[i-1] {
			t.Fatalf("keyframes not strictly increasing: %v", times)
		}
	}
}

// TestGomediaKeyframesDefersCopyDecision checks that a go-media timeline with a
// gap past the copy cap is not used for the pre-flight decision (ffprobe must
// confirm it), while a plain probe still returns it.
func TestGomediaKeyframesDefersCopyDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes a clip with ffmpeg")
	}
	if settings.AlwaysUseFfmpegEnabled() {
		t.Skip("go-media disabled by settings")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	path := filepath.Join(t.TempDir(), "longgop.mp4")
	out, err := exec.Command("ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=128x96:rate=25:duration=40",
		"-c:v", "libx264", "-g", "750", "-keyint_min", "750", "-sc_threshold", "0", "-pix_fmt", "yuv420p", path).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg cannot encode libx264: %v: %s", err, out)
	}

	if times, ok := gomediaKeyframes(context.Background(), nil, path, 4, true); ok {
		t.Fatalf("pre-flight must defer a 30s GOP to ffprobe, got %v", times)
	}
	times, ok := gomediaKeyframes(context.Background(), nil, path, 4, false)
	if !ok {
		t.Fatal("a plain probe must return the go-media timeline")
	}
	if len(times) != 2 || times[0] != 0 || times[1] < 29.9 || times[1] > 30.1 {
		t.Fatalf("unexpected keyframes %v, want [0 30]", times)
	}
}
