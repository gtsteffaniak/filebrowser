package transcoding

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type slowBuildEngine struct {
	FakeEngine
	buildStarted chan struct{}
	releaseBuild chan struct{}
}

func (s *slowBuildEngine) BuildPlan(ctx context.Context, realPath string, profile Profile) (hlsPlan, error) {
	s.buildStarted <- struct{}{}
	<-s.releaseBuild
	return s.FakeEngine.BuildPlan(ctx, realPath, profile)
}

type blockingStartEngine struct {
	FakeEngine
	job *fakeJob
}

func (b *blockingStartEngine) StartContinuous(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, startSec float64) (ContinuousJob, error) {
	if err := os.MkdirAll(filepath.Join(outputDir, "seg"), 0o755); err != nil {
		return nil, err
	}
	return b.job, nil
}

func TestManagerReuseSameFileRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewFakeEngine()
	mgr, err := NewManager(ctx, engine, store, Options{BaseURL: ""})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	in := StartInput{
		Username:  "u",
		UserLimit: 1,
		Source:    "s",
		Path:      "/v.mp4",
		RealPath:  t.TempDir() + "/v.mp4",
		Profile:   ProfileQuality,
		ClientID:  "c1",
	}
	first, err := mgr.Start(ctx, in)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	second, err := mgr.Start(ctx, in)
	if err != nil {
		t.Fatalf("reuse: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("expected same session, got %s then %s", first.ID, second.ID)
	}
	if !second.Reused {
		t.Fatal("expected reused=true")
	}
}

func TestManagerUserLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewFakeEngine()
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	base := StartInput{
		Username:  "user-two",
		UserLimit: 1,
		Source:    "s",
		Path:      "/a.mp4",
		RealPath:  t.TempDir() + "/a.mp4",
		Profile:   ProfileQuality,
		ClientID:  "c1",
	}
	if _, err := mgr.Start(ctx, base); err != nil {
		t.Fatalf("first: %v", err)
	}
	other := base
	other.Path = "/b.mp4"
	other.RealPath = t.TempDir() + "/b.mp4"
	other.ClientID = "c2"
	if _, err := mgr.Start(ctx, other); err != ErrUserLimit {
		t.Fatalf("want user limit, got %v", err)
	}
}

func TestManagerConcurrentSameKeySingleSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewFakeEngine()
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	var wg sync.WaitGroup
	const n = 20
	ids := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			info, err := mgr.Start(ctx, StartInput{
				Username:  "user-three",
				UserLimit: 1,
				Source:    "s",
				Path:      "/same.mp4",
				RealPath:  t.TempDir() + "/same.mp4",
				Profile:   ProfileQuality,
				ClientID:  "client",
			})
			if err != nil {
				t.Errorf("start: %v", err)
				return
			}
			ids <- info.ID
		}()
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		} else if id != first {
			t.Fatalf("expected one session id, also saw %s", id)
		}
	}
}

func TestManagerStopIdempotent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(ctx, NewFakeEngine(), store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	info, err := mgr.Start(ctx, StartInput{
		Username: "user-four", UserLimit: 1, Source: "s", Path: "/x.mp4",
		RealPath: t.TempDir() + "/x.mp4", Profile: ProfileQuality,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Stop("user-four", info.ID); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Stop("user-four", info.ID); err != ErrNotFound {
		t.Fatalf("second stop: %v", err)
	}
}

func TestManagerHeartbeatRequiresSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, _ := NewStore(t.TempDir())
	mgr, _ := NewManager(ctx, NewFakeEngine(), store, Options{})
	defer mgr.Close(context.Background())

	if err := mgr.Heartbeat("user-one", "missing", HeartbeatRequest{}); err != ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestStoreRejectsInvalidSegment(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	jobDir := t.TempDir()
	if _, err := store.SegmentPath(jobDir, "../etc/passwd"); err == nil {
		t.Fatal("expected invalid segment error")
	}
	if _, err := store.SegmentPath(jobDir, "bad.m4s"); err == nil {
		t.Fatal("expected invalid segment name")
	}
}

func TestSessionReusable(t *testing.T) {
	running := &playbackSession{state: StateRunning}
	// A generation that starts at segment 0 covers the whole file, so a
	// completed session serves any playhead without transcoding again.
	completedFromStart := &playbackSession{state: StateCompleted, coversFromStart: true}
	// A generation that started mid-file never produced earlier segments.
	completedPartial := &playbackSession{state: StateCompleted, startSec: 60}
	runningPartial := &playbackSession{state: StateRunning, startSec: 60}
	failed := &playbackSession{state: StateFailed}

	if !sessionReusable(running, 12) {
		t.Fatal("running session should be reusable while playing")
	}
	if !sessionReusable(completedFromStart, 0) {
		t.Fatal("completed session should be reusable from start")
	}
	if !sessionReusable(completedFromStart, 12) {
		t.Fatal("a completed generation that covers the file should serve a later seek")
	}
	if sessionReusable(completedPartial, 12) {
		t.Fatal("a mid-file generation must not be reused for an earlier playhead")
	}
	if !sessionReusable(completedPartial, 90) {
		t.Fatal("a mid-file generation should serve a later playhead")
	}
	if sessionReusable(runningPartial, 12) {
		t.Fatal("a mid-file running generation must not be reused for an earlier playhead")
	}
	if !sessionReusable(runningPartial, 90) {
		t.Fatal("a mid-file running generation should serve a later playhead")
	}
	if sessionReusable(failed, 0) {
		t.Fatal("failed session should not be reused")
	}
}

func TestRewritePlaylistRewritesDeliveryURLs(t *testing.T) {
	mgr := &Manager{baseURL: "/testing"}
	playlist := "#EXTM3U\n#EXT-X-MAP:URI=\"init.m4s\"\n#EXTINF:4,\nseg/00000.m4s\n"
	got := string(mgr.rewritePlaylist("session", []byte(playlist)))
	want := "#EXTM3U\n" +
		"#EXT-X-MAP:URI=\"/testing/api/media/transcode/sessions/session/init.m4s\"\n" +
		"#EXTINF:4,\n" +
		"/testing/api/media/transcode/sessions/session/seg/00000.m4s\n"
	if got != want {
		t.Fatalf("playlist mismatch\n got: %q\nwant: %q", got, want)
	}
}

func TestManagerJanitorDoesNotRace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store, _ := NewStore(t.TempDir())
	mgr, _ := NewManager(ctx, NewFakeEngine(), store, Options{})
	defer func() {
		shCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = mgr.Close(shCtx)
	}()
	time.Sleep(janitorInterval + 20*time.Millisecond)
}

func TestSessionInfoOmitsUserID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(ctx, NewFakeEngine(), store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	info, err := mgr.Start(ctx, StartInput{
		Username:  "alice",
		UserLimit: 2,
		Source:    "s",
		Path:      "/clip.mp4",
		RealPath:  t.TempDir() + "/clip.mp4",
		Profile:   ProfileQuality,
	})
	if err != nil {
		t.Fatal(err)
	}
	if info.Username != "alice" {
		t.Fatalf("username=%q", info.Username)
	}
}

func TestManagerSupersedesNonReusableSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewFakeEngine()
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	base := StartInput{
		Username:  "supersede-user",
		UserLimit: 1,
		Source:    "s",
		Path:      "/clip.mp4",
		RealPath:  t.TempDir() + "/clip.mp4",
		Profile:   ProfileQuality,
		ClientID:  "c1",
	}
	first, err := mgr.Start(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	mgr.mu.Lock()
	sess := mgr.sessions[first.ID]
	sess.state = StateCompleted
	sess.info.State = StateCompleted
	// Simulate a generation that started mid-file: it never produced the earlier
	// segments, so a request for an earlier playhead must start a new session.
	sess.startSec = 60
	sess.coversFromStart = false
	mgr.mu.Unlock()

	second, err := mgr.Start(ctx, StartInput{
		Username:  base.Username,
		UserLimit: base.UserLimit,
		Source:    base.Source,
		Path:      base.Path,
		RealPath:  base.RealPath,
		Profile:   base.Profile,
		ClientID:  "c2",
		StartSec:  12,
	})
	if err != nil {
		t.Fatalf("superseding start: %v", err)
	}
	if second.ID == first.ID {
		t.Fatal("expected a new session after non-reusable supersession")
	}

	snap := mgr.Snapshot(base.Username, false, base.UserLimit)
	if snap.UserActive != 1 {
		t.Fatalf("userActive=%d want 1 after supersession", snap.UserActive)
	}
	if snap.GlobalActive != 1 {
		t.Fatalf("globalActive=%d want 1 after supersession", snap.GlobalActive)
	}
	if _, err := mgr.SessionForUser(base.Username, first.ID); err != ErrNotFound {
		t.Fatalf("old session should be gone, got %v", err)
	}
}

// A replace target that already expired (janitor expiry, producer failure) must
// not fail the start. No slot was released in that case, so the admission limits
// still apply.
func TestManagerStartToleratesExpiredReplaceSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(ctx, NewFakeEngine(), store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	start := func(path string, userLimit int, replace string) (SessionInfo, error) {
		t.Helper()
		return mgr.Start(ctx, StartInput{
			Username:         "expired-user",
			UserLimit:        userLimit,
			Source:           "s",
			Path:             path,
			RealPath:         t.TempDir() + path,
			Profile:          ProfileQuality,
			ReplaceSessionID: replace,
			ClientID:         path,
		})
	}

	if _, err := start("/a.mp4", 1, ""); err != nil {
		t.Fatalf("first start: %v", err)
	}

	// A different file means no reuse, so this exercises the replace branch.
	if _, err := start("/b.mp4", 2, "does-not-exist"); err != nil {
		t.Fatalf("an expired replace target must not fail the start: %v", err)
	}

	// Nothing was replaced, so the per-user limit still applies.
	if _, err := start("/c.mp4", 2, "also-missing"); err != ErrUserLimit {
		t.Fatalf("want ErrUserLimit when no slot was released, got %v", err)
	}
}

// toggleEngine reports availability that can change after construction, standing
// in for ffmpeg becoming usable at runtime.
type toggleEngine struct {
	FakeEngine
	available atomic.Bool
}

func (e *toggleEngine) Available() bool { return e.available.Load() }

// Availability is evaluated per request, so a manager created while ffmpeg was
// unavailable starts serving as soon as the engine reports available. Wiring the
// manager only when ffmpeg is present at startup would pin transcoding off until
// the next restart.
func TestManagerEnabledIsEvaluatedPerRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := &toggleEngine{}
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	if mgr.Enabled() {
		t.Fatal("manager must report disabled while the engine is unavailable")
	}
	engine.available.Store(true)
	if !mgr.Enabled() {
		t.Fatal("manager must report enabled once the engine becomes available")
	}
	engine.available.Store(false)
	if mgr.Enabled() {
		t.Fatal("manager must report disabled again when the engine goes away")
	}
}

func TestManagerProducersKeyedBySessionID(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewFakeEngine()
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	start := func(path string) SessionInfo {
		t.Helper()
		info, err := mgr.Start(ctx, StartInput{
			Username:  "producer-user",
			UserLimit: 2,
			Source:    "s",
			Path:      path,
			RealPath:  t.TempDir() + path,
			Profile:   ProfileQuality,
		})
		if err != nil {
			t.Fatalf("start %s: %v", path, err)
		}
		return info
	}

	first := start("/a.mp4")
	second := start("/b.mp4")

	deadline := time.Now().Add(2 * time.Second)
	for {
		mgr.mu.Lock()
		ready := len(mgr.producers) == 2 &&
			mgr.producers[first.ID] != nil &&
			mgr.producers[second.ID] != nil
		mgr.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for both session producers")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestManagerBuildPlanOutsideLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := &slowBuildEngine{
		buildStarted: make(chan struct{}, 1),
		releaseBuild: make(chan struct{}),
	}
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	started := make(chan SessionInfo, 1)
	go func() {
		info, err := mgr.Start(ctx, StartInput{
			Username:  "slow-user",
			UserLimit: 1,
			Source:    "s",
			Path:      "/slow.mp4",
			RealPath:  t.TempDir() + "/slow.mp4",
			Profile:   ProfileQuality,
		})
		if err != nil {
			t.Errorf("start: %v", err)
			close(started)
			return
		}
		started <- info
	}()

	select {
	case <-engine.buildStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for BuildPlan to start")
	}

	if err := mgr.Heartbeat("slow-user", "missing", HeartbeatRequest{}); err != ErrNotFound {
		t.Fatalf("heartbeat during BuildPlan should not block, got %v", err)
	}

	close(engine.releaseBuild)

	select {
	case info := <-started:
		if info.ID == "" {
			t.Fatal("expected session id")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for session start to finish")
	}
}

func TestManagerCloseWaitsForProducers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	blockingJob := NewBlockingFakeJob()
	engine := &blockingStartEngine{job: blockingJob}
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}

	info, err := mgr.Start(ctx, StartInput{
		Username:  "close-user",
		UserLimit: 1,
		Source:    "s",
		Path:      "/close.mp4",
		RealPath:  t.TempDir() + "/close.mp4",
		Profile:   ProfileQuality,
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		mgr.mu.Lock()
		ready := mgr.producers[info.ID] != nil
		mgr.mu.Unlock()
		if ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for producer registration")
		}
		time.Sleep(10 * time.Millisecond)
	}

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer closeCancel()
	if err := mgr.Close(closeCtx); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-blockingJob.Cancelled():
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not cancel running producer")
	}
}

// A completed generation on disk is shared by every user playing the same file
// with the same profile: no second transcode, and the cached playlist is served.
func TestManagerAttachesToCompletedGenerationAcrossUsers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewFakeEngine()
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	realPath := t.TempDir() + "/clip.mp4"
	plan, planErr := engine.BuildPlan(ctx, realPath, ProfileQuality)
	if planErr != nil {
		t.Fatal(planErr)
	}
	// Pre-seed the cache as if another user had already finished this transcode.
	generationDir := filepath.Join(store.root, plan.cacheFingerprint, "gen-existing")
	writeCompleteGeneration(t, generationDir, 9000)

	start := func(username string) SessionInfo {
		t.Helper()
		info, startErr := mgr.Start(ctx, StartInput{
			Username:  username,
			UserLimit: 1,
			Source:    "s",
			Path:      "/clip.mp4",
			RealPath:  realPath,
			Profile:   ProfileQuality,
			ClientID:  username,
		})
		if startErr != nil {
			t.Fatalf("start %s: %v", username, startErr)
		}
		return info
	}

	first := start("user-a")
	second := start("user-b")

	if engine.StartCount() != 0 {
		t.Fatalf("expected no new ffmpeg producer, got %d starts", engine.StartCount())
	}
	for _, info := range []SessionInfo{first, second} {
		if info.State != StateCompleted {
			t.Fatalf("session %s state = %s, want completed", info.ID, info.State)
		}
	}
	jobDir, err := mgr.JobDir("user-a", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if jobDir != generationDir {
		t.Fatalf("jobDir = %q, want the cached generation %q", jobDir, generationDir)
	}
	// The cached playlist is served to both users.
	playlist, err := mgr.Playlist("user-b", second.ID)
	if err != nil {
		t.Fatalf("playlist: %v", err)
	}
	if !strings.Contains(string(playlist), "/seg/00000.m4s") {
		t.Fatalf("playlist missing cached segment: %q", string(playlist))
	}
}

// While one user's transcode is still running, a second user starting the same
// file and profile attaches to that producer instead of spawning another ffmpeg.
func TestManagerSharesInProgressProducerAcrossUsers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewFakeEngine()
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	realPath := t.TempDir() + "/shared.mp4"
	in := StartInput{
		Username:  "user-a",
		UserLimit: 1,
		Source:    "s",
		Path:      "/shared.mp4",
		RealPath:  realPath,
		Profile:   ProfileQuality,
		ClientID:  "a",
	}
	first, err := mgr.Start(ctx, in)
	if err != nil {
		t.Fatal(err)
	}

	in.Username = "user-b"
	in.ClientID = "b"
	second, err := mgr.Start(ctx, in)
	if err != nil {
		t.Fatalf("second start: %v", err)
	}

	// The producer runs in its own goroutine; wait for it to start before
	// asserting that no second ffmpeg process was launched.
	deadline := time.Now().Add(2 * time.Second)
	for engine.StartCount() < 1 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the first producer to start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if engine.StartCount() != 1 {
		t.Fatalf("expected a single ffmpeg producer, got %d", engine.StartCount())
	}
	firstDir, err := mgr.JobDir("user-a", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	secondDir, err := mgr.JobDir("user-b", second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if firstDir != secondDir {
		t.Fatalf("sessions share no generation: %q vs %q", firstDir, secondDir)
	}

	mgr.mu.Lock()
	producer := mgr.producers[first.ID]
	shared := mgr.producers[second.ID]
	refs := 0
	if producer != nil {
		refs = producer.refs
	}
	mgr.mu.Unlock()
	if producer == nil || producer != shared {
		t.Fatal("expected both sessions to share one producer")
	}
	if refs != 2 {
		t.Fatalf("producer refs = %d, want 2", refs)
	}

	// Stopping one viewer must not stop the shared transcode.
	if err := mgr.Stop("user-b", second.ID); err != nil {
		t.Fatalf("stop user-b: %v", err)
	}
	mgr.mu.Lock()
	stillRunning := mgr.producers[first.ID]
	mgr.mu.Unlock()
	if stillRunning == nil {
		t.Fatal("shared producer was dropped while another session still watches")
	}
}

// Stopping the last viewer of a generation releases the shared producer.
func TestManagerLastViewerReleasesSharedProducer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	engine := NewFakeEngine()
	mgr, err := NewManager(ctx, engine, store, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close(context.Background())

	in := StartInput{
		Username:  "user-a",
		UserLimit: 1,
		Source:    "s",
		Path:      "/last.mp4",
		RealPath:  t.TempDir() + "/last.mp4",
		Profile:   ProfileQuality,
		ClientID:  "a",
	}
	first, err := mgr.Start(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Username = "user-b"
	in.ClientID = "b"
	second, err := mgr.Start(ctx, in)
	if err != nil {
		t.Fatal(err)
	}

	if err := mgr.Stop("user-b", second.ID); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Stop("user-a", first.ID); err != nil {
		t.Fatal(err)
	}
	mgr.mu.Lock()
	remaining := len(mgr.producers)
	producing := len(mgr.producing)
	mgr.mu.Unlock()
	if remaining != 0 || producing != 0 {
		t.Fatalf("producers=%d producing=%d, want both empty", remaining, producing)
	}
}
