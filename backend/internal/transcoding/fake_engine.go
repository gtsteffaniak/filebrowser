package transcoding

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// FakeEngine is a deterministic Engine for tests.
type FakeEngine struct {
	mu        sync.Mutex
	Starts    int
	plan      hlsPlan
	lastIndex int
	lastSec   float64
	lastPlan  hlsPlan
	lastJob   *fakeJob
	jobDirs   []string
}

func NewFakeEngine() *FakeEngine {
	return &FakeEngine{
		plan: hlsPlan{
			cacheFingerprint: "fp-test",
			segmentSec:       4,
			mediaDurationSec: 10,
		},
	}
}

func (f *FakeEngine) Available() bool { return true }

// StartCount returns how many ffmpeg jobs have been launched.
func (f *FakeEngine) StartCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Starts
}

func (f *FakeEngine) BuildPlan(ctx context.Context, realPath string, profile Profile) (hlsPlan, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	plan := f.plan
	// Mirror production: the cache fingerprint is keyed by file identity and
	// transcode parameters only, never by user or session.
	plan.cacheFingerprint = "fp-" + filepath.Base(realPath) + "-" + string(profile)
	return plan, nil
}

type fakeJob struct {
	cancelled  chan struct{}
	waitErr    error
	unreliable atomic.Bool
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

func (j *fakeJob) VTDecodeUnreliable() bool {
	return j.unreliable.Load()
}

// SetVTUnreliable simulates ffmpeg reporting sustained VideoToolbox decode failures.
func (j *fakeJob) SetVTUnreliable() {
	j.unreliable.Store(true)
}

// NewBlockingFakeJob returns a ContinuousJob that waits until Cancel is called.
func NewBlockingFakeJob() *fakeJob {
	return newFakeJob()
}

// Cancelled exposes the cancel signal for tests.
func (j *fakeJob) Cancelled() <-chan struct{} {
	return j.cancelled
}

// LastStart reports the segment index and ffmpeg -ss offset of the most recent
// continuous job, for tests that assert reposition alignment.
func (f *FakeEngine) LastStart() (startIndex int, startSec float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastIndex, f.lastSec
}

// LastPlan returns the plan passed to the most recent StartContinuous call, so
// tests can assert decode-profile changes across retries.
func (f *FakeEngine) LastPlan() hlsPlan {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastPlan
}

// LastJob returns the job created by the most recent StartContinuous call.
func (f *FakeEngine) LastJob() *fakeJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastJob
}

func (f *FakeEngine) StartContinuous(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, startSec float64) (ContinuousJob, error) {
	f.mu.Lock()
	f.Starts++
	f.lastIndex = startIndex
	f.lastSec = startSec
	f.lastPlan = plan
	job := newFakeJob()
	f.lastJob = job
	f.jobDirs = append(f.jobDirs, outputDir)
	f.mu.Unlock()
	_ = os.MkdirAll(filepath.Join(outputDir, "seg"), 0o755)
	_ = os.WriteFile(filepath.Join(outputDir, "init.m4s"), []byte("fake-init-segment-bytes-for-test"), 0o644)
	_ = os.WriteFile(filepath.Join(outputDir, "index.m3u8"), []byte("#EXTM3U\n"), 0o644)
	return job, nil
}
