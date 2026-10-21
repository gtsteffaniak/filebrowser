package transcoding

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
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
	vodList := ffmpeg.BuildVODSegmentList(10, 4, 0, nil)
	return &FakeEngine{
		plan: hlsPlan{
			cacheFingerprint: "fp-test",
			segmentSec:       4,
			mediaDurationSec: 10,
			vodList:          vodList,
			playlistBody:     ffmpeg.BuildVODPlaylist(vodList, ffmpeg.DefaultVODPlaylistOptions()),
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
	doneMu     sync.Mutex
	waitErr    error
	unreliable atomic.Bool
}

func newFakeJob() *fakeJob {
	return &fakeJob{cancelled: make(chan struct{})}
}

func (j *fakeJob) Cancel() {
	j.setDone(context.Canceled)
}

// Complete finishes the job cleanly, as if ffmpeg ran to EOF on its own.
func (j *fakeJob) Complete() {
	j.setDone(nil)
}

func (j *fakeJob) setDone(err error) {
	j.doneMu.Lock()
	defer j.doneMu.Unlock()
	select {
	case <-j.cancelled:
	default:
		j.waitErr = err
		close(j.cancelled)
	}
}

func (j *fakeJob) Wait() error {
	<-j.cancelled
	return j.waitErr
}

// Running reports whether the fake job is still active (non-blocking).
func (j *fakeJob) Running() bool {
	select {
	case <-j.cancelled:
		return false
	default:
		return true
	}
}

func (j *fakeJob) VTDecodeUnreliable() bool {
	return j.unreliable.Load()
}

// SetVTUnreliable simulates ffmpeg reporting sustained VideoToolbox decode failures.
func (j *fakeJob) SetVTUnreliable() {
	j.unreliable.Store(true)
}

// NewBlockingFakeJob returns a VODJob that waits until Cancel is called.
func NewBlockingFakeJob() *fakeJob {
	return newFakeJob()
}

// Cancelled exposes the cancel signal for tests.
func (j *fakeJob) Cancelled() <-chan struct{} {
	return j.cancelled
}

// LastStart reports the segment index of the most recent VOD job, for tests
// that assert reposition alignment.
func (f *FakeEngine) LastStart() (startIndex int, startSec float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastIndex, f.lastSec
}

// LastPlan returns the plan passed to the most recent StartVODJob call, so
// tests can assert decode-profile changes across retries.
func (f *FakeEngine) LastPlan() hlsPlan {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastPlan
}

// LastJob returns the job created by the most recent StartVODJob call.
func (f *FakeEngine) LastJob() *fakeJob {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lastJob
}

// StartVODJob launches a fake job anchored at startIndex. The session arg is
// unused: a fake job cannot be installed into an HLSVODSession (that requires a
// concrete *ops.HLSVODSegmentJob), so the manager drives decisions from the
// producer-level job view instead.
func (f *FakeEngine) StartVODJob(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, session *ffmpeg.HLSVODSession) (VODJob, error) {
	f.mu.Lock()
	f.Starts++
	f.lastIndex = startIndex
	f.lastSec = vodPlaylistStartSec(plan, startIndex)
	f.lastPlan = plan
	job := newFakeJob()
	f.lastJob = job
	f.jobDirs = append(f.jobDirs, outputDir)
	f.mu.Unlock()
	_ = os.MkdirAll(filepath.Join(outputDir, "seg"), 0o755)
	_ = os.WriteFile(filepath.Join(outputDir, "init.m4s"), []byte("fake-init-segment-bytes-for-test"), 0o644)
	playlist := plan.playlistBody
	if playlist == "" {
		playlist = "#EXTM3U\n"
	}
	_ = os.WriteFile(filepath.Join(outputDir, "index.m3u8"), []byte(playlist), 0o644)
	_ = os.WriteFile(filepath.Join(outputDir, "seg", fmt.Sprintf("%05d.m4s", startIndex)), fakeVODSegmentPayload(), 0o644)
	return job, nil
}
