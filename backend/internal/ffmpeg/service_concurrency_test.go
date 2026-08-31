package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func requireFFmpegBinaries(t *testing.T) {
	t.Helper()
	if os.Getenv("GOFFMPEG_FFMPEG_PATH") != "" {
		return
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}
}

func testSampleMP4(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("GOFFMPEG_SAMPLE_MP4"); p != "" {
		return p
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "sample.mp4")
	cmd := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=blue:s=320x240:d=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-y", out)
	if err := cmd.Run(); err != nil {
		t.Skipf("could not generate sample mp4: %v", err)
	}
	return out
}

// TestGetMediaDurationConcurrentNoDeadlock ensures probe-tier concurrency does not deadlock.
// With MaxProbe=2, four concurrent GetMediaDuration calls queue and complete.
func TestGetMediaDurationConcurrentNoDeadlock(t *testing.T) {
	requireFFmpegBinaries(t)
	sample := testSampleMP4(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := Initialize(ctx, InitOptions{
		Concurrency: goffmpeg.Concurrency{
			MaxProbe:  2,
			MaxDecode: 4,
			MaxEncode: 2,
		},
		CacheDir:             t.TempDir(),
		SkipHWTests:          true,
		HardwareAcceleration: false,
	})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	t.Cleanup(func() { global = nil })

	svc := Get()
	if svc == nil {
		t.Fatal("Get() returned nil after Initialize")
	}

	const workers = 4
	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.GetMediaDuration(ctx, sample); err != nil {
				errCh <- err
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(25 * time.Second):
		t.Fatal("concurrent GetMediaDuration calls deadlocked with MaxProbe=2")
	}

	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("GetMediaDuration() error = %v", err)
		}
	}
}
