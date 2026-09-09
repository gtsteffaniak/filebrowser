package transcoding

import (
	"context"
	"os"
	"path/filepath"
	"sync"
)

// FakeEngine is a deterministic Engine for tests.
type FakeEngine struct {
	mu     sync.Mutex
	Starts int
	plan   hlsPlan
}

func NewFakeEngine() *FakeEngine {
	return &FakeEngine{
		plan: hlsPlan{
			cacheFingerprint: "fp-test",
			segmentSec:       4,
			segmentDurations: []float64{4, 4, 2},
		},
	}
}

func (f *FakeEngine) Available() bool { return true }

func (f *FakeEngine) BuildPlan(ctx context.Context, realPath string, profile Profile) (hlsPlan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.plan, nil
}

type fakeJob struct {
	cancelled chan struct{}
	waitErr   error
}

func newFakeJob() *fakeJob {
	return &fakeJob{cancelled: make(chan struct{})}
}

func (j *fakeJob) Cancel() {
	select {
	case <-j.cancelled:
	default:
		close(j.cancelled)
	}
}

func (j *fakeJob) Wait() error {
	<-j.cancelled
	return j.waitErr
}

// NewBlockingFakeJob returns a ContinuousJob that waits until Cancel is called.
func NewBlockingFakeJob() *fakeJob {
	return newFakeJob()
}

// Cancelled exposes the cancel signal for tests.
func (j *fakeJob) Cancelled() <-chan struct{} {
	return j.cancelled
}

func (f *FakeEngine) StartContinuous(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, startSec float64) (ContinuousJob, error) {
	f.mu.Lock()
	f.Starts++
	f.mu.Unlock()
	_ = os.MkdirAll(filepath.Join(outputDir, "seg"), 0o755)
	_ = os.WriteFile(filepath.Join(outputDir, "init.m4s"), []byte("fake-init-segment-bytes-for-test"), 0o644)
	_ = os.WriteFile(filepath.Join(outputDir, "ffmpeg.m3u8"), []byte("#EXTM3U\n"), 0o644)
	return newFakeJob(), nil
}
