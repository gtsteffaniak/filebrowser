package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/mediacache"
)

const cacheFixtureMP4 = "../../../frontend/tests/playwright-files/myfolder/testdata/sample-640x480.mp4"

var (
	testServiceOnce sync.Once
	testServiceErr  error
)

// initTestService starts the ffmpeg service once, without hardware probes, or
// skips.
func initTestService(t *testing.T) *Service {
	t.Helper()
	if testing.Short() {
		t.Skip("starts ffmpeg")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	testServiceOnce.Do(func() {
		testServiceErr = Initialize(context.Background(), InitOptions{SkipHWTests: true, CacheDir: os.TempDir()})
	})
	if testServiceErr != nil {
		t.Skipf("ffmpeg init failed: %v", testServiceErr)
	}
	svc := Get()
	if svc == nil {
		t.Skip("ffmpeg service unavailable")
	}
	return svc
}

// copyFixture copies the MP4 fixture so the test owns its cache key.
func copyFixture(t *testing.T) (string, mediacache.Key) {
	t.Helper()
	data, err := os.ReadFile(cacheFixtureMP4)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "sample.mp4")
	if err = os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	key, err := mediacache.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		mediacache.Probes.Delete(key.String())
		mediacache.Durations.Delete(key.String())
	})
	return path, key
}

// TestProbeFileUsesSharedCache checks that ffprobe results go into the shared
// cache, cached values are reused, and WithoutCache bypasses both.
func TestProbeFileUsesSharedCache(t *testing.T) {
	svc := initTestService(t)
	path, key := copyFixture(t)

	got, err := svc.ProbeFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	cached, ok := mediacache.Probes.Get(key.String())
	if !ok || cached != *got {
		t.Fatalf("ProbeFile must store its result in the shared cache: cached=%+v ok=%v got=%+v", cached, ok, *got)
	}

	sentinel := FileProbeInfo{Duration: 1, VideoCodec: "sentinel"}
	mediacache.Probes.Set(key.String(), sentinel)
	got, err = svc.ProbeFile(context.Background(), path)
	if err != nil || *got != sentinel {
		t.Fatalf("ProbeFile must reuse a cached value (e.g. from go-media): got=%+v err=%v", got, err)
	}

	fresh, err := svc.ProbeFile(mediacache.WithoutCache(context.Background()), path)
	if err != nil || *fresh == sentinel {
		t.Fatalf("WithoutCache must probe fresh: got=%+v err=%v", fresh, err)
	}
	if cached, _ := mediacache.Probes.Get(key.String()); cached != sentinel {
		t.Fatal("WithoutCache must not overwrite the shared cache")
	}
}

// TestGetMediaDurationUsesSharedCache checks that cached durations are reused
// and WithoutCache probes fresh.
func TestGetMediaDurationUsesSharedCache(t *testing.T) {
	svc := initTestService(t)
	path, key := copyFixture(t)

	mediacache.Durations.Set(key.String(), 4242)
	d, err := svc.GetMediaDuration(context.Background(), path)
	if err != nil || d != 4242 {
		t.Fatalf("GetMediaDuration must reuse a cached value: d=%v err=%v", d, err)
	}
	d, err = svc.GetMediaDuration(mediacache.WithoutCache(context.Background()), path)
	if err != nil || d == 4242 || d <= 0 {
		t.Fatalf("WithoutCache must probe fresh: d=%v err=%v", d, err)
	}
}
