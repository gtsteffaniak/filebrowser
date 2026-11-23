package ffmpeg

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"

	"github.com/gtsteffaniak/filebrowser/backend/internal/mediacache"
	"github.com/gtsteffaniak/filebrowser/backend/internal/testutil"
)

const testFixtureMP4 = "../../../frontend/tests/playwright-files/myfolder/testdata/sample-640x480.mp4"

var (
	integrationServiceOnce sync.Once
	integrationServiceErr  error
)

func requireFFmpegBinaries(t *testing.T) {
	t.Helper()
	testutil.SkipIfNoFFmpeg(t)
}

// initIntegrationService starts the global ffmpeg service once for subprocess
// integration tests. Hardware encoder smoke probes are skipped here; functional
// HW coverage lives in go-ffmpeg, not filebrowser unit CI.
func initIntegrationService(t *testing.T) *Service {
	t.Helper()
	if testing.Short() {
		t.Skip("starts ffmpeg")
	}
	requireFFmpegBinaries(t)
	integrationServiceOnce.Do(func() {
		integrationServiceErr = Initialize(context.Background(), InitOptions{
			SkipHWTests:          true,
			CacheDir:             os.TempDir(),
			HardwareAcceleration: false,
			EncoderHierarchy:     []capabilities.AccelType{capabilities.AccelNone},
			Concurrency: goffmpeg.Concurrency{
				MaxProbe:  2,
				MaxDecode: 4,
				MaxEncode: 2,
			},
		})
	})
	if integrationServiceErr != nil {
		t.Skipf("ffmpeg init failed: %v", integrationServiceErr)
	}
	svc := Get()
	if svc == nil {
		t.Skip("ffmpeg service unavailable")
	}
	return svc
}

// copyTestFixtureMP4 copies the checked-in sample so the test owns its cache key.
func copyTestFixtureMP4(t *testing.T) (string, mediacache.Key) {
	t.Helper()
	if p := os.Getenv("GOFFMPEG_SAMPLE_MP4"); p != "" {
		key, err := mediacache.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		return p, key
	}
	data, err := os.ReadFile(testFixtureMP4)
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