package transcoding

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
)

func TestManagerReuseSameFileRefresh(t *testing.T) {
	mgr, _ := newTestManager(t, nil)
	in := testInput(t, "u", "/v.mp4")

	first := mustStart(t, mgr, in)
	second := mustStart(t, mgr, in)
	if first.ID != second.ID {
		t.Fatalf("expected same session, got %s then %s", first.ID, second.ID)
	}
	if !second.Reused {
		t.Fatal("expected reused=true")
	}
}

func TestManagerUserLimit(t *testing.T) {
	mgr, _ := newTestManager(t, nil)

	mustStart(t, mgr, testInput(t, "user-two", "/a.mp4"))
	other := testInput(t, "user-two", "/b.mp4")
	other.ClientID = "c2"
	if _, err := mgr.Start(context.Background(), other); err != ErrUserLimit {
		t.Fatalf("want user limit, got %v", err)
	}
}

func TestManagerConcurrentSameKeySingleSession(t *testing.T) {
	mgr, _ := newTestManager(t, nil)
	in := testInput(t, "user-three", "/same.mp4")
	in.ClientID = "client"

	var wg sync.WaitGroup
	const n = 20
	ids := make(chan string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			info, err := mgr.Start(context.Background(), in)
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

// An abandoned session (every client lease expired) must be reaped within
// orphanSessionGrace, not held for the whole reconnectGrace window. A stale
// heartbeat is what makes the leases expire here.
func TestManagerReapsAbandonedSessionQuickly(t *testing.T) {
	mgr, _ := newTestManager(t, nil)
	sess := mustStart(t, mgr, testInput(t, "abandoner", "/a.mp4"))

	// Leases must be past reconnectGrace to be pruned, which is the real
	// "client is gone" signal; lastHeartbeat past orphanSessionGrace then makes
	// the session itself reclaimable.
	leaseStale := time.Now().Add(-reconnectGrace - time.Second)
	heartbeatStale := time.Now().Add(-orphanSessionGrace - time.Second)
	mgr.mu.Lock()
	live := mgr.sessions[sess.ID]
	if live == nil {
		mgr.mu.Unlock()
		t.Fatal("session vanished before setup")
	}
	for id := range live.clients {
		live.clients[id] = clientLease{seen: leaseStale, paused: true}
	}
	live.lastHeartbeat = heartbeatStale
	live.lastActivity = heartbeatStale
	// Model a slow-starting session: the old code exempted StateStarting
	// sessions for the whole startupTimeout, holding the slot.
	live.state = StateStarting
	live.info.State = StateStarting
	mgr.mu.Unlock()

	mgr.expireSessions()

	mgr.mu.Lock()
	_, still := mgr.sessions[sess.ID]
	_, producerStill := mgr.producers[sess.ID]
	mgr.mu.Unlock()
	if still || producerStill {
		t.Fatal("an abandoned session must be reaped within orphanSessionGrace")
	}
}

// A session whose client is merely between heartbeats (recent lastHeartbeat)
// must survive the janitor so a slow tick does not kill live playback.
func TestManagerKeepsSessionWithRecentHeartbeat(t *testing.T) {
	mgr, _ := newTestManager(t, nil)
	sess := mustStart(t, mgr, testInput(t, "fresh", "/fresh.mp4"))

	mgr.mu.Lock()
	live := mgr.sessions[sess.ID]
	staleLease := time.Now().Add(-reconnectGrace - time.Second)
	for id := range live.clients {
		live.clients[id] = clientLease{seen: staleLease, paused: true}
	}
	// The lease just expired, but the heartbeat was moments ago.
	live.lastHeartbeat = time.Now()
	live.lastActivity = time.Now()
	mgr.mu.Unlock()

	mgr.expireSessions()

	mgr.mu.Lock()
	_, still := mgr.sessions[sess.ID]
	mgr.mu.Unlock()
	if !still {
		t.Fatal("a session with a fresh heartbeat must not be reaped")
	}
}

func TestManagerStopIdempotent(t *testing.T) {
	mgr, _ := newTestManager(t, nil)

	info := mustStart(t, mgr, testInput(t, "user-four", "/x.mp4"))
	if err := mgr.Stop("user-four", info.ID); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Stop("user-four", info.ID); err != ErrNotFound {
		t.Fatalf("second stop: %v", err)
	}
}

func TestManagerHeartbeatRequiresSession(t *testing.T) {
	mgr, _ := newTestManager(t, nil)

	if err := mgr.Heartbeat("user-one", "missing", HeartbeatRequest{}); err != ErrNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestSessionReusable(t *testing.T) {
	mgr, _ := newTestManager(t, nil)
	reusable := func(sess *playbackSession, startSec float64) bool {
		mgr.mu.Lock()
		defer mgr.mu.Unlock()
		return mgr.sessionReusableLocked(sess, startSec)
	}

	plan := hlsPlan{
		segmentSec:       4,
		mediaDurationSec: 300,
		vodList:          ffmpeg.BuildVODSegmentList(300, 4, 0, nil),
	}
	running := &playbackSession{state: StateRunning, plan: plan}
	// A generation that starts at segment 0 covers the whole file, so a
	// completed session serves any playhead without transcoding again.
	completedFromStart := &playbackSession{state: StateCompleted, plan: plan}
	// A generation that started at segment 15 never produced earlier segments.
	completedPartial := &playbackSession{state: StateCompleted, startSec: 60, startIndex: 15, plan: plan}
	runningPartial := &playbackSession{state: StateRunning, startSec: 60, startIndex: 15, plan: plan}
	failed := &playbackSession{state: StateFailed, plan: plan}

	if !reusable(running, 12) {
		t.Fatal("running session should be reusable for an in-bounds playhead")
	}
	if !reusable(running, 130) {
		t.Fatal("a session with no live producer serves any in-bounds target")
	}
	if !reusable(running, 300) {
		t.Fatal("a target at the media end should still reuse the session")
	}
	if reusable(running, 400) {
		t.Fatal("an out-of-bounds target must not reuse the session")
	}
	if !reusable(completedFromStart, 0) {
		t.Fatal("completed session should be reusable from start")
	}
	if !reusable(completedFromStart, 12) {
		t.Fatal("a completed generation that covers the file should serve a later seek")
	}
	if reusable(completedPartial, 12) {
		t.Fatal("a mid-file generation must not be reused for an earlier playhead")
	}
	if !reusable(completedPartial, 90) {
		t.Fatal("a completed mid-file generation should serve a later playhead")
	}
	if reusable(runningPartial, 12) {
		t.Fatal("a mid-file running generation must not be reused for an earlier playhead")
	}
	if !reusable(runningPartial, 90) {
		t.Fatal("a mid-file running generation should serve a later playhead")
	}
	if reusable(failed, 0) {
		t.Fatal("failed session should not be reused")
	}

	// Sessions on a live producer reuse only inside its servable window: a
	// target beyond the encode head plus the forward gap would reposition the
	// shared job, so that viewer gets its own generation instead.
	live := &playbackSession{state: StateRunning, plan: plan}
	live.info.ID = "live-producer"
	mgr.producers["live-producer"] = &producer{
		plan:       plan,
		jobDir:     t.TempDir(),
		startIndex: 0,
		encodeHead: 10,
	}
	if !reusable(live, 40) {
		t.Fatal("a target inside the produced range should reuse the live generation")
	}
	if !reusable(live, 60) {
		t.Fatal("a target inside the forward wait gap should reuse the live generation")
	}
	if reusable(live, 130) {
		t.Fatal("a target beyond the forward gap must not reuse — it gets its own generation")
	}

	// Below the job start a target still reuses when a previous run left the
	// segment ready on disk — no reposition is needed to serve it.
	jobDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(jobDir, "seg"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeServeableSegment(t, filepath.Join(jobDir, "seg"), "00005.m4s")
	backward := &playbackSession{state: StateRunning, plan: plan}
	backward.info.ID = "backward-producer"
	mgr.producers["backward-producer"] = &producer{
		plan:       plan,
		jobDir:     jobDir,
		startIndex: 10,
		encodeHead: 12,
	}
	if !reusable(backward, 20) {
		t.Fatal("a ready segment below the job start should reuse the live generation")
	}
	if reusable(backward, 24) {
		t.Fatal("a missing segment below the job start must not reuse")
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
	newTestManager(t, nil)
	time.Sleep(janitorInterval + 20*time.Millisecond)
}

func TestSessionInfoOmitsUserID(t *testing.T) {
	mgr, _ := newTestManager(t, nil)

	in := testInput(t, "alice", "/clip.mp4")
	in.UserLimit = 2
	info := mustStart(t, mgr, in)
	if info.Username != "alice" {
		t.Fatalf("username=%q", info.Username)
	}
	// The session id is an opaque random token used to address the session and its
	// delivery URLs, never a user identifier.
	if len(info.ID) != 32 {
		t.Fatalf("session id %q should be a 32-char opaque token", info.ID)
	}
	if _, err := hex.DecodeString(info.ID); err != nil {
		t.Fatalf("session id %q should be hex, not a user id: %v", info.ID, err)
	}

	for _, payload := range []any{info, mgr.Snapshot("alice", false, 2)} {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if key := userIDKeyPattern.Find(encoded); key != nil {
			t.Fatalf("session payload exposes a user id field %s: %s", key, encoded)
		}
	}
}

func TestManagerSupersedesNonReusableSession(t *testing.T) {
	mgr, _ := newTestManager(t, nil)

	base := testInput(t, "supersede-user", "/clip.mp4")
	first := mustStart(t, mgr, base)
	// Simulate a generation that started mid-file: it never produced the earlier
	// segments, so a request for an earlier playhead must start a new session.
	seedSessionState(t, mgr, first.ID, func(sess *playbackSession) {
		sess.state = StateCompleted
		sess.info.State = StateCompleted
		sess.startSec = 60
		sess.startIndex = 15
	})

	second := base
	second.ClientID = "c2"
	second.StartSec = 12
	second.ReplaceSessionID = first.ID
	info, err := mgr.Start(context.Background(), second)
	if err != nil {
		t.Fatalf("superseding start: %v", err)
	}
	if info.ID == first.ID {
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
	mgr, _ := newTestManager(t, nil)

	start := func(path string, userLimit int, replace string) (SessionInfo, error) {
		t.Helper()
		in := testInput(t, "expired-user", path)
		in.UserLimit = userLimit
		in.ReplaceSessionID = replace
		in.ClientID = path
		return mgr.Start(context.Background(), in)
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

// Switching profiles for the same user+client+file must hand over the existing
// session even when replaceSessionId is omitted (frontend race).
func TestManagerStartHandsOverSameFileClientSession(t *testing.T) {
	mgr, _ := newTestManager(t, nil)

	base := testInput(t, "handover-user", "/clip.mp4")
	base.Profile = ProfileDataSaver
	base.ClientID = "plyr-1"
	first := mustStart(t, mgr, base)

	second := base
	second.Profile = ProfileBalanced
	// Intentionally omit ReplaceSessionID — the manager must still hand over.
	info, err := mgr.Start(context.Background(), second)
	if err != nil {
		t.Fatalf("balanced start without replaceSessionId: %v", err)
	}
	if info.ID == first.ID {
		t.Fatal("expected a new session for the new profile")
	}
	if info.Profile != ProfileBalanced {
		t.Fatalf("profile = %s, want balanced", info.Profile)
	}
	if _, err := mgr.SessionForUser(base.Username, first.ID); err != ErrNotFound {
		t.Fatalf("old datasaver session should be gone, got %v", err)
	}
	snap := mgr.Snapshot(base.Username, false, base.UserLimit)
	if snap.UserActive != 1 {
		t.Fatalf("userActive=%d want 1 after handover", snap.UserActive)
	}
}

// Reusing another client's completed same-file session must still release this
// client's live session for a different profile so the slot does not leak.
func TestManagerReuseReleasesOtherSameFileClientSession(t *testing.T) {
	mgr, _ := newTestManager(t, nil)

	base := testInput(t, "reuse-handover", "/clip.mp4")
	base.UserLimit = 2
	base.Profile = ProfileBalanced
	base.ClientID = "other-client"
	balanced := mustStart(t, mgr, base)
	seedSessionState(t, mgr, balanced.ID, func(sess *playbackSession) {
		sess.state = StateCompleted
		sess.info.State = StateCompleted
	})

	datasaverIn := base
	datasaverIn.Profile = ProfileDataSaver
	datasaverIn.ClientID = "plyr-1"
	datasaver := mustStart(t, mgr, datasaverIn)

	// plyr-1 switches to balanced: reuse the completed session and release datasaver.
	reuse := datasaverIn
	reuse.Profile = ProfileBalanced
	reuse.UserLimit = 1
	reused, err := mgr.Start(context.Background(), reuse)
	if err != nil {
		t.Fatalf("reuse balanced: %v", err)
	}
	if !reused.Reused || reused.ID != balanced.ID {
		t.Fatalf("expected reused balanced session, got id=%s reused=%v", reused.ID, reused.Reused)
	}
	if _, err := mgr.SessionForUser("reuse-handover", datasaver.ID); err != ErrNotFound {
		t.Fatalf("datasaver session should be released after reuse handover, got %v", err)
	}
}

// Availability is evaluated per request, so a manager created while ffmpeg was
// unavailable starts serving as soon as the engine reports available. Wiring the
// manager only when ffmpeg is present at startup would pin transcoding off until
// the next restart.
func TestManagerEnabledIsEvaluatedPerRequest(t *testing.T) {
	engine := &toggleEngine{}
	mgr, _ := newTestManager(t, engine)

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
	mgr, _ := newTestManager(t, nil)

	start := func(path string) SessionInfo {
		t.Helper()
		in := testInput(t, "producer-user", path)
		in.UserLimit = 2
		return mustStart(t, mgr, in)
	}
	first := start("/a.mp4")
	second := start("/b.mp4")

	pollUntil(t, 2*time.Second, func() bool {
		mgr.mu.Lock()
		defer mgr.mu.Unlock()
		return len(mgr.producers) == 2 &&
			mgr.producers[first.ID] != nil &&
			mgr.producers[second.ID] != nil
	}, "both session producers")
}

func TestManagerBuildPlanOutsideLock(t *testing.T) {
	engine := &slowBuildEngine{
		buildStarted: make(chan struct{}, 1),
		releaseBuild: make(chan struct{}),
	}
	mgr, _ := newTestManager(t, engine)
	in := testInput(t, "slow-user", "/slow.mp4")

	started := make(chan SessionInfo, 1)
	go func() {
		info, err := mgr.Start(context.Background(), in)
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
	blockingJob := NewBlockingFakeJob()
	engine := &blockingStartEngine{job: blockingJob}
	mgr, _ := newTestManager(t, engine)

	info := mustStart(t, mgr, testInput(t, "close-user", "/close.mp4"))
	pollUntil(t, 2*time.Second, func() bool {
		return producerFor(t, mgr, info.ID) != nil
	}, "producer registration")

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
	engine := NewFakeEngine()
	mgr, store := newTestManager(t, engine)

	realPath := filepath.Join(t.TempDir(), "clip.mp4")
	plan, planErr := engine.BuildPlan(context.Background(), realPath, ProfileQuality)
	if planErr != nil {
		t.Fatal(planErr)
	}
	// Pre-seed the cache as if another user had already finished this transcode.
	generationDir := filepath.Join(store.root, plan.cacheFingerprint, "gen-existing")
	writeCompleteGeneration(t, generationDir, 9000)

	start := func(username string) SessionInfo {
		t.Helper()
		in := testInput(t, username, "/clip.mp4")
		in.RealPath = realPath
		in.ClientID = username
		return mustStart(t, mgr, in)
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
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "user-a", "/shared.mp4")
	in.ClientID = "a"
	first := mustStart(t, mgr, in)
	in.Username = "user-b"
	in.ClientID = "b"
	second := mustStart(t, mgr, in)

	// The producer runs in its own goroutine; wait for it to start before
	// asserting that no second ffmpeg process was launched.
	waitForStarts(t, engine, 1)
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

	producer := producerFor(t, mgr, first.ID)
	if producer == nil || producer != producerFor(t, mgr, second.ID) {
		t.Fatal("expected both sessions to share one producer")
	}
	if snap := snapshotProducer(t, mgr, first.ID); snap.refs != 2 {
		t.Fatalf("producer refs = %d, want 2", snap.refs)
	}

	// Stopping one viewer must not stop the shared transcode.
	if err := mgr.Stop("user-b", second.ID); err != nil {
		t.Fatalf("stop user-b: %v", err)
	}
	if producerFor(t, mgr, first.ID) == nil {
		t.Fatal("shared producer was dropped while another session still watches")
	}
}

// Stopping the last viewer of a generation releases the shared producer.
func TestManagerLastViewerReleasesSharedProducer(t *testing.T) {
	mgr, _ := newTestManager(t, nil)

	in := testInput(t, "user-a", "/last.mp4")
	in.ClientID = "a"
	first := mustStart(t, mgr, in)
	in.Username = "user-b"
	in.ClientID = "b"
	second := mustStart(t, mgr, in)

	if err := mgr.Stop("user-b", second.ID); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Stop("user-a", first.ID); err != nil {
		t.Fatal(err)
	}
	mgr.mu.Lock()
	remaining := len(mgr.producers)
	mgr.mu.Unlock()
	if remaining != 0 {
		t.Fatalf("producers=%d, want empty", remaining)
	}
}

// A seek beyond the produced edge must supersede the requester's session with
// a new generation aligned to the seek target — the whole point of
// replaceSessionId repositioning.
func TestManagerRepositionsSeekBeyondProducedEdge(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "seeker", "/v.mp4")
	first := mustStart(t, mgr, in)
	// Segments 0..4 produced: edge at 20s on the 4s grid.
	genDir := sessionJobDir(t, mgr, first.ID)
	waitForPlaylistFile(t, genDir)
	writeProducedRange(t, genDir, 0, 5, false)

	second := in
	second.StartSec = 300
	second.ReplaceSessionID = first.ID
	info, err := mgr.Start(context.Background(), second)
	if err != nil {
		t.Fatalf("reposition start: %v", err)
	}
	if info.ID == first.ID {
		t.Fatal("a seek past the produced edge must start a new generation")
	}
	waitForStarts(t, engine, 2)
	// VOD Model A snaps the seek target to the synthetic playlist (10s fixture).
	if idx, sec := engine.LastStart(); idx != 2 || sec != 8 {
		t.Fatalf("new generation start = (%d, %v), want (2, 8)", idx, sec)
	}
	if _, err := mgr.SessionForUser(in.Username, first.ID); err != ErrNotFound {
		t.Fatalf("old session should be superseded, got %v", err)
	}
}

// A seek inside the produced range must reuse the live session — no restart.
func TestManagerReusesWithinProducedCoverage(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "viewer", "/v.mp4")
	in.UserLimit = 2
	first := mustStart(t, mgr, in)
	genDir := sessionJobDir(t, mgr, first.ID)
	waitForPlaylistFile(t, genDir)
	writeProducedRange(t, genDir, 0, 5, false)

	reused := in
	reused.UserLimit = 0
	reused.ClientID = "c2"
	reused.StartSec = 10
	info, err := mgr.Start(context.Background(), reused)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != first.ID || !info.Reused {
		t.Fatalf("seek inside coverage should reuse the session, got id=%q reused=%v", info.ID, info.Reused)
	}
	if engine.StartCount() != 1 {
		t.Fatalf("ffmpeg starts=%d, want 1", engine.StartCount())
	}
}

// A different client seeking beyond coverage gets its own generation — the
// shared session must not be superseded out from under its other clients.
func TestManagerOutOfRangeSeekKeepsOtherClientSession(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "multi", "/v.mp4")
	in.UserLimit = 4
	first := mustStart(t, mgr, in)
	genDir := sessionJobDir(t, mgr, first.ID)
	waitForPlaylistFile(t, genDir)
	writeProducedRange(t, genDir, 0, 5, false)

	second := in
	second.ClientID = "c2"
	second.StartSec = 300
	info, err := mgr.Start(context.Background(), second)
	if err != nil {
		t.Fatalf("second client start: %v", err)
	}
	if info.ID == first.ID {
		t.Fatal("the second client must get its own session")
	}
	if _, err := mgr.SessionForUser(in.Username, first.ID); err != nil {
		t.Fatalf("the first session must survive another client's seek: %v", err)
	}
	waitForStarts(t, engine, 2)
}

// A viewer starting far beyond the shared job's servable window gets its own
// generation — attaching it would let its segment requests restart the shared
// job out from under the other viewers.
func TestManagerFarViewerGetsOwnGeneration(t *testing.T) {
	engine := NewFakeEngine()
	list := ffmpeg.BuildVODSegmentList(300, 4, 0, nil)
	engine.plan.vodList = list
	engine.plan.mediaDurationSec = 300
	engine.plan.playlistBody = ffmpeg.BuildVODPlaylist(list, ffmpeg.DefaultVODPlaylistOptions())
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "user-a", "/v.mp4")
	in.UserLimit = 4
	first := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)

	second := in
	second.Username = "user-b"
	second.ClientID = "b"
	second.StartSec = 200
	info, err := mgr.Start(context.Background(), second)
	if err != nil {
		t.Fatalf("far viewer start: %v", err)
	}
	if info.ID == first.ID {
		t.Fatal("far viewer must not reuse the early session")
	}
	waitForStarts(t, engine, 2)
	if sessionJobDir(t, mgr, first.ID) == sessionJobDir(t, mgr, info.ID) {
		t.Fatal("far viewer must get its own generation dir")
	}
	if producerFor(t, mgr, first.ID) == producerFor(t, mgr, info.ID) {
		t.Fatal("far viewer must not share the early producer")
	}
}

// A segment-request restart is deferred while another attached session's
// playhead sits inside the job's servable window and the job still has
// reposition cooldown left — repositioning sooner would pull the encode out
// from under them before it produced anything there. The requester's own
// reposition goes through once the cooldown lapses or the rider goes stale.
func TestSignalVODRestartSkipsWhileViewerRidesWindow(t *testing.T) {
	mgr, _ := newTestManager(t, nil)
	plan := hlsPlan{
		segmentSec:       4,
		mediaDurationSec: 300,
		vodList:          ffmpeg.BuildVODSegmentList(300, 4, 0, nil),
	}
	p := &producer{
		plan:            plan,
		jobDir:          t.TempDir(),
		startIndex:      0,
		encodeHead:      10,
		vodRestartIndex: -1,
		job:             newFakeJob(),
		jobStartedAt:    time.Now(),
		resume:          make(chan struct{}, 1),
	}
	rider := &playbackSession{plan: plan, playheadSec: 40, lastHeartbeat: time.Now()}
	rider.info.ID = "rider"
	far := &playbackSession{plan: plan}
	far.info.ID = "far"
	mgr.sessions["rider"] = rider
	mgr.sessions["far"] = far
	mgr.producers["rider"] = p
	mgr.producers["far"] = p

	mgr.signalVODRestart(far, p, 70)
	if p.vodRestartIndex >= 0 {
		t.Fatal("restart must not land while another viewer rides the window")
	}
	if !p.job.Running() {
		t.Fatal("the shared job must not be cancelled for a far viewer's request")
	}

	// Once the job has had its reposition cooldown, a contested request wins:
	// produced coverage survives the reposition, so the rider converges back.
	p.jobStartedAt = time.Now().Add(-vodRepositionCooldown - time.Second)
	mgr.signalVODRestart(far, p, 70)
	if p.vodRestartIndex != 70 {
		t.Fatalf("restart index = %d, want 70 once the cooldown lapsed", p.vodRestartIndex)
	}
	if p.job.Running() {
		t.Fatal("an unblocked restart must cancel the running job")
	}

	// A duplicate restart for the already-repositioned index is a no-op so
	// polling while the new job starts cannot cancel the replacement job.
	p.vodRestartIndex = -1
	p.job = newFakeJob()
	mgr.signalVODRestart(far, p, 70)
	if !p.job.Running() {
		t.Fatal("duplicate restart for covered index must not cancel the new job")
	}

	// A stale rider stops blocking: dead heartbeats are not demand.
	p.startIndex = 0
	p.encodeHead = 10
	p.vodRestartIndex = -1
	p.job = newFakeJob()
	p.jobStartedAt = time.Now()
	rider.lastHeartbeat = time.Now().Add(-reconnectGrace - time.Second)
	mgr.signalVODRestart(far, p, 70)
	if p.vodRestartIndex != 70 {
		t.Fatalf("restart index = %d, want 70 once the rider went stale", p.vodRestartIndex)
	}
	if p.job.Running() {
		t.Fatal("an unblocked restart must cancel the running job")
	}
}

// A job that ends cleanly before covering the whole timeline — a bounded
// restart stopping at an already-ready tail — parks for demand-driven resume
// instead of finalizing an incomplete generation.
func TestManagerParksProducerOnIncompleteJobEnd(t *testing.T) {
	engine := NewFakeEngine()
	list := ffmpeg.BuildVODSegmentList(300, 4, 0, nil)
	engine.plan.vodList = list
	engine.plan.mediaDurationSec = 300
	engine.plan.playlistBody = ffmpeg.BuildVODPlaylist(list, ffmpeg.DefaultVODPlaylistOptions())
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "parker", "/v.mp4")
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)

	genDir := sessionJobDir(t, mgr, sess.ID)
	writeProducedRange(t, genDir, 0, 5, false)
	pollUntil(t, 2*time.Second, func() bool {
		return snapshotProducer(t, mgr, sess.ID).encodeHead >= 5
	}, "encode head to advance over produced range")

	engine.LastJob().Complete()
	pollUntil(t, 2*time.Second, func() bool {
		return snapshotProducer(t, mgr, sess.ID).suspended
	}, "producer to park on incomplete coverage")
	if got := sessionState(t, mgr, sess.ID); got == StateCompleted || got == StateFailed {
		t.Fatalf("incomplete generation must not finalize, state=%s", got)
	}

	// Demand for a missing segment relaunches the producer at that index.
	f, err := mgr.OpenSegment(context.Background(), in.Username, sess.ID, "00008.m4s")
	if err != nil {
		t.Fatalf("segment request on parked producer: %v", err)
	}
	f.Close()
	waitForStarts(t, engine, 2)
	if idx, _ := engine.LastStart(); idx != 8 {
		t.Fatalf("resumed job start index = %d, want 8", idx)
	}
}

// A completed VOD generation is reused for any in-range playhead without ffmpeg.
func TestManagerAttachesCoveringCompletedGeneration(t *testing.T) {
	engine := NewFakeEngine()
	mgr, store := newTestManager(t, engine)

	realPath := filepath.Join(t.TempDir(), "v.mp4")
	plan, err := engine.BuildPlan(context.Background(), realPath, ProfileQuality)
	if err != nil {
		t.Fatal(err)
	}
	writeProducedRange(t, store.JobDir(plan.cacheFingerprint, "gen-full"), 0, 3, true)

	in := testInput(t, "u", "/v.mp4")
	in.UserLimit = 4
	in.RealPath = realPath
	in.ClientID = "seek-in-range"
	in.StartSec = 5
	attached := mustStart(t, mgr, in)
	if attached.State != StateCompleted {
		t.Fatalf("attached session state = %q, want completed", attached.State)
	}
	if !attached.VodPlaylist {
		t.Fatal("expected Model A VOD session")
	}
	if engine.StartCount() != 0 {
		t.Fatalf("a covering cache hit must not start ffmpeg, starts=%d", engine.StartCount())
	}
	if attached.StartSec != 5 {
		t.Fatalf("published session startSec = %v, want 5 (hls startPosition)", attached.StartSec)
	}
	if attached.EndSec != 10 {
		t.Fatalf("published session endSec = %v, want 10", attached.EndSec)
	}

	fresh := in
	fresh.ClientID = "seek-from-start"
	fresh.StartSec = 0
	info, err := mgr.Start(context.Background(), fresh)
	if err != nil {
		t.Fatal(err)
	}
	if info.State != StateCompleted || engine.StartCount() != 0 {
		t.Fatalf("start from 0 should attach completed cache, state=%q starts=%d", info.State, engine.StartCount())
	}
}

// A session that completes after being created as running publishes the final
// produced end, so a client attaching later can serve any playhead inside the
// generation without repositioning.
func TestManagerCompletionPublishesCoverageEnd(t *testing.T) {
	engine := &blockingStartEngine{job: newFakeJob()}
	engine.plan = NewFakeEngine().plan
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "u", "/v.mp4")
	in.UserLimit = 4
	info := mustStart(t, mgr, in)
	// VOD playlists expose the full media duration up front.
	if info.EndSec != 10 {
		t.Fatalf("fresh session endSec = %v, want 10", info.EndSec)
	}

	// Three emitted 4s segments (12s of produced media) with ENDLIST.
	writeProducedRange(t, sessionJobDir(t, mgr, info.ID), 0, 3, true)
	engine.job.Complete()

	pollUntil(t, 5*time.Second, func() bool {
		got, err := mgr.SessionForUser("u", info.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.State == StateCompleted
	}, "session completion")
	got, err := mgr.SessionForUser("u", info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.EndSec != 10 {
		t.Fatalf("completed session endSec = %v, want 10 (full media duration)", got.EndSec)
	}
}

// Sustained hardware decode failures must restart the generation once with
// software decode, keeping the segments already produced and resuming at the
// encode head — wiping the generation would strand viewers that are already
// playing, and restarting at the session start would re-encode the whole file.
// A second failure does not loop restarts.
func TestManagerRestartsProducerWithSoftwareDecode(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	info := mustStart(t, mgr, testInput(t, "user-a", "/movie.mp4"))
	waitForStarts(t, engine, 1)

	// Seed output resembling produced coverage from the failing run.
	jobDir := sessionJobDir(t, mgr, info.ID)
	writeProducedRange(t, jobDir, 0, 3, false)
	pollUntil(t, 2*time.Second, func() bool {
		return snapshotProducer(t, mgr, info.ID).encodeHead >= 3
	}, "encode head to advance past the produced segments")

	engine.LastJob().SetVTUnreliable()

	waitForStarts(t, engine, 2)
	if got := engine.LastPlan().decodeProfile; !got.ForceSoftware {
		t.Fatalf("retry decode profile = %+v, want ForceSoftware", got)
	}
	// Resume at the encode head, not the session's original start.
	if idx, _ := engine.LastStart(); idx != 3 {
		t.Fatalf("retry start index = %d, want 3 (the encode head)", idx)
	}
	// Already-produced segments survive the retry so viewers keep playing.
	for i := 0; i < 3; i++ {
		segPath := filepath.Join(jobDir, "seg", fmt.Sprintf("%05d.m4s", i))
		if _, err := os.Stat(segPath); err != nil {
			t.Fatalf("segment %d was removed by the decode retry: %v", i, err)
		}
	}
	if sess := sessionFor(t, mgr, info.ID); sess == nil {
		t.Fatal("session should survive the decode retry")
	}

	// A second unreliable flag must not trigger another restart.
	engine.LastJob().SetVTUnreliable()
	time.Sleep(500 * time.Millisecond)
	if got := engine.StartCount(); got != 2 {
		t.Fatalf("starts = %d, want exactly 2 (decode retry is once-only)", got)
	}
}

// ---- test helpers ----

// slowBuildEngine blocks inside BuildPlan until releaseBuild closes, standing
// in for a slow ffprobe so tests can assert the manager lock is not held.
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

// blockingStartEngine returns a job that runs until cancelled, so a producer
// stays registered for the duration of the test.
type blockingStartEngine struct {
	FakeEngine
	job *fakeJob
}

func (b *blockingStartEngine) StartVODJob(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, session *ffmpeg.HLSVODSession) (VODJob, error) {
	if err := os.MkdirAll(filepath.Join(outputDir, "seg"), 0o755); err != nil {
		return nil, err
	}
	return b.job, nil
}

// toggleEngine reports availability that can change after construction, standing
// in for ffmpeg becoming usable at runtime.
type toggleEngine struct {
	FakeEngine
	available atomic.Bool
}

func (e *toggleEngine) Available() bool { return e.available.Load() }

// newTestManager wires a Store in a temp dir and a manager around engine (a
// FakeEngine when nil), closing the manager at test cleanup.
// All clients paused past the grace window suspends the producer: the job is
// cancelled, produced segments stay servable, and the session is not finalized.
func TestManagerSuspendsProducerWhenAllClientsPaused(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "pauser", "/v.mp4")
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	job := engine.LastJob()

	if err := mgr.Heartbeat(in.Username, sess.ID, HeartbeatRequest{ClientID: "c1", Paused: true}); err != nil {
		t.Fatal(err)
	}
	suspendPausedProducers(t, mgr, time.Now().Add(time.Hour))

	pollUntil(t, 2*time.Second, func() bool { return !job.Running() }, "suspended job to stop")
	if snap := snapshotProducer(t, mgr, sess.ID); !snap.exists || !snap.suspended {
		t.Fatal("a suspended producer must stay registered and marked suspended")
	}
	if got := sessionState(t, mgr, sess.ID); got == StateFailed || got == StateCompleted {
		t.Fatalf("pause suspension must not finalize the session, state=%s", got)
	}
	// Already-produced segments still serve without churning the producer.
	f, err := mgr.OpenSegment(context.Background(), in.Username, sess.ID, "00000.m4s")
	if err != nil {
		t.Fatalf("ready segment after suspend: %v", err)
	}
	f.Close()
	if engine.StartCount() != 1 {
		t.Fatalf("serving a ready segment must not relaunch ffmpeg, starts=%d", engine.StartCount())
	}
}

// One playing client anywhere on a producer keeps it alive — suspension only
// happens when every attached client is paused.
func TestManagerKeepsProducerWhileAnyClientPlays(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "multi", "/v.mp4")
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	job := engine.LastJob()

	if err := mgr.Heartbeat(in.Username, sess.ID, HeartbeatRequest{ClientID: "c1", Paused: true}); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Heartbeat(in.Username, sess.ID, HeartbeatRequest{ClientID: "c2", Paused: false}); err != nil {
		t.Fatal(err)
	}
	suspendPausedProducers(t, mgr, time.Now().Add(time.Hour))

	if !job.Running() {
		t.Fatal("a playing client must prevent producer suspension")
	}
	if snap := snapshotProducer(t, mgr, sess.ID); !snap.exists || snap.suspended {
		t.Fatal("producer must not be suspended while a client plays")
	}
}

// Inside the grace window the producer keeps running — a quick pause/unpause
// must not churn ffmpeg restarts.
func TestManagerDoesNotSuspendWithinPauseGrace(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "quick", "/v.mp4")
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	job := engine.LastJob()

	if err := mgr.Heartbeat(in.Username, sess.ID, HeartbeatRequest{ClientID: "c1", Paused: true}); err != nil {
		t.Fatal(err)
	}
	suspendPausedProducers(t, mgr, time.Now())

	if !job.Running() {
		t.Fatal("producer must survive a pause shorter than the grace window")
	}
}

// An un-pause heartbeat wakes the suspended producer — ffmpeg relaunches
// before the next segment request lands.
func TestManagerResumesProducerOnUnpauseHeartbeat(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "pauser", "/v.mp4")
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	// The fake wrote segment 0 at launch; wait until the encode head sees it
	// so the resume target is deterministic.
	pollUntil(t, 2*time.Second, func() bool {
		return snapshotProducer(t, mgr, sess.ID).encodeHead >= 1
	}, "encode head to advance past segment 0")
	job := engine.LastJob()

	if err := mgr.Heartbeat(in.Username, sess.ID, HeartbeatRequest{ClientID: "c1", Paused: true}); err != nil {
		t.Fatal(err)
	}
	suspendPausedProducers(t, mgr, time.Now().Add(time.Hour))
	pollUntil(t, 2*time.Second, func() bool { return !job.Running() }, "suspended job to stop")

	if err := mgr.Heartbeat(in.Username, sess.ID, HeartbeatRequest{ClientID: "c1", Paused: false}); err != nil {
		t.Fatal(err)
	}
	waitForStarts(t, engine, 2)
	if snap := snapshotProducer(t, mgr, sess.ID); !snap.exists || snap.suspended {
		t.Fatal("producer must be live after an un-pause heartbeat")
	}
	// Resume relaunches at the encode head — not the generation's start.
	if idx, _ := engine.LastStart(); idx != 1 {
		t.Fatalf("resumed job start index = %d, want 1 (encode head)", idx)
	}
}

// A segment request while suspended relaunches the producer at the requested
// index via the existing VOD restart path — the cross-platform pause is just
// cancel + the restart demand already uses for seeks.
func TestManagerResumesProducerOnSegmentRequest(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "pauser", "/v.mp4")
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	job := engine.LastJob()

	if err := mgr.Heartbeat(in.Username, sess.ID, HeartbeatRequest{ClientID: "c1", Paused: true}); err != nil {
		t.Fatal(err)
	}
	suspendPausedProducers(t, mgr, time.Now().Add(time.Hour))
	pollUntil(t, 2*time.Second, func() bool { return !job.Running() }, "suspended job to stop")

	// The suspended job reads as !Running, so a request beyond its head takes
	// the restart path: the relaunched job produces the requested segment.
	f, err := mgr.OpenSegment(context.Background(), in.Username, sess.ID, "00002.m4s")
	if err != nil {
		t.Fatalf("segment request after suspend: %v", err)
	}
	f.Close()
	waitForStarts(t, engine, 2)
	if idx, _ := engine.LastStart(); idx != 2 {
		t.Fatalf("restarted job start index = %d, want 2", idx)
	}
}

// Stopping a session whose producer is parked releases it cleanly — Close must
// not block on a goroutine parked waiting for resume.
func TestManagerStopReleasesSuspendedProducer(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "pauser", "/v.mp4")
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	job := engine.LastJob()

	if err := mgr.Heartbeat(in.Username, sess.ID, HeartbeatRequest{ClientID: "c1", Paused: true}); err != nil {
		t.Fatal(err)
	}
	suspendPausedProducers(t, mgr, time.Now().Add(time.Hour))
	pollUntil(t, 2*time.Second, func() bool { return !job.Running() }, "suspended job to stop")

	if err := mgr.Stop(in.Username, sess.ID); err != nil {
		t.Fatal(err)
	}
	if p := producerFor(t, mgr, sess.ID); p != nil {
		t.Fatal("stopping a suspended session must detach the producer")
	}
	// Manager close would hang if the parked producer goroutine never woke.
	if err := mgr.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func newTestManager(t *testing.T, engine Engine) (*Manager, *Store) {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if engine == nil {
		engine = NewFakeEngine()
	}
	mgr, err := NewManager(context.Background(), engine, store, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close(context.Background()) })
	return mgr, store
}

// testInput returns a StartInput for username+path with the defaults most tests
// share; override fields (StartSec, ReplaceSessionID, ...) as needed.
func testInput(t *testing.T, username, path string) StartInput {
	t.Helper()
	return StartInput{
		Username:  username,
		UserLimit: 1,
		Source:    "s",
		Path:      path,
		RealPath:  filepath.Join(t.TempDir(), path),
		Profile:   ProfileQuality,
		ClientID:  "c1",
	}
}

// mustStart calls mgr.Start and fails the test on error.
func mustStart(t *testing.T, mgr *Manager, in StartInput) SessionInfo {
	t.Helper()
	info, err := mgr.Start(context.Background(), in)
	if err != nil {
		t.Fatalf("start %s: %v", in.Path, err)
	}
	return info
}

// pollUntil fails the test when cond is not met within timeout.
func pollUntil(t *testing.T, timeout time.Duration, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", msg)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitForStarts polls the fake engine until n ffmpeg jobs have launched —
// producers run on their own goroutine, so the count is not synchronous.
func waitForStarts(t *testing.T, engine *FakeEngine, n int) {
	t.Helper()
	pollUntil(t, 5*time.Second, func() bool {
		return engine.StartCount() >= n
	}, fmt.Sprintf("%d ffmpeg starts (have %d)", n, engine.StartCount()))
}

// suspendPausedProducers runs the janitor's suspension pass as if `now` were
// the current time, so tests can jump past the pause grace window.
func suspendPausedProducers(t *testing.T, mgr *Manager, now time.Time) {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.suspendPausedProducersLocked(now)
}

// seedSessionState mutates a session under the manager lock, for tests that
// need a state the producer lifecycle has not reached yet.
func seedSessionState(t *testing.T, mgr *Manager, id string, mutate func(*playbackSession)) {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mutate(mgr.sessions[id])
}

// sessionFor returns the internal session under the manager lock.
func sessionFor(t *testing.T, mgr *Manager, id string) *playbackSession {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.sessions[id]
}

// producerFor returns the producer registered for a session under the lock.
// Only pointer identity is safe to check on the result — mutable fields race
// the producer loop; use snapshotProducer to read them.
func producerFor(t *testing.T, mgr *Manager, id string) *producer {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.producers[id]
}

// producerSnapshot carries the producer fields tests assert on. Fields are
// zero and exists is false when the session has no registered producer.
type producerSnapshot struct {
	exists     bool
	suspended  bool
	encodeHead int
	refs       int
}

// snapshotProducer reads the session's producer fields under the manager lock
// so assertions don't race the producer goroutine.
func snapshotProducer(t *testing.T, mgr *Manager, id string) producerSnapshot {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	p := mgr.producers[id]
	if p == nil {
		return producerSnapshot{}
	}
	return producerSnapshot{
		exists:     true,
		suspended:  p.suspended,
		encodeHead: p.encodeHead,
		refs:       p.refs,
	}
}

// sessionState returns the session's lifecycle state under the manager lock.
func sessionState(t *testing.T, mgr *Manager, id string) SessionState {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.sessions[id].state
}

func sessionJobDir(t *testing.T, mgr *Manager, sessionID string) string {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.sessions[sessionID].jobDir
}

// waitForPlaylistFile waits for the producer's initial playlist to land so a
// later write of simulated coverage cannot be clobbered by the fake's own write.
func waitForPlaylistFile(t *testing.T, genDir string) {
	t.Helper()
	playlistPath := filepath.Join(genDir, "index.m3u8")
	pollUntil(t, 5*time.Second, func() bool {
		st, err := os.Stat(playlistPath)
		return err == nil && st.Size() > 0
	}, "the producer's initial playlist")
}

// writeProducedRange writes a generation's produced output — seg/ files plus
// index.m3u8 covering [firstIdx, firstIdx+count) — with the vod.complete
// marker when endList is set so coverage checks see a finished generation.
func writeProducedRange(t *testing.T, genDir string, firstIdx, count int, endList bool) {
	t.Helper()
	segDir := filepath.Join(genDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(genDir, "init.m4s"), fakeInitSegment(), 0o644); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:4\n")
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", firstIdx)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%05d.m4s", firstIdx+i)
		writeServeableSegment(t, segDir, name)
		fmt.Fprintf(&b, "#EXTINF:4.000000,\nseg/%s\n", name)
	}
	if endList {
		b.WriteString("#EXT-X-ENDLIST\n")
	}
	if err := os.WriteFile(filepath.Join(genDir, "index.m3u8"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if endList {
		if err := os.WriteFile(filepath.Join(genDir, vodCompleteMarker), fmt.Appendf(nil, "%g\n", float64(count)*4), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// userIDKeyPattern matches a JSON key that looks like a user identifier, at any
// depth — the guard TestSessionInfoOmitsUserID applies to session payloads.
var userIDKeyPattern = regexp.MustCompile(`(?i)"user[-_ ]?id"\s*:`)

// A mid-file job that reaches EOF without producing the prefix must park (not
// finalize) so a seek back to segment 0 can reposition the producer.
func TestManagerParksWhenMidFileEOFLeavesPrefixMissing(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "seek-back", "/v.mp4")
	in.StartSec = 4
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	genDir := sessionJobDir(t, mgr, sess.ID)
	waitForPlaylistFile(t, genDir)

	// Job started at index 1; simulate ffmpeg running through EOF with segs 1–2 only.
	writeProducedRange(t, genDir, 1, 2, false)
	pollUntil(t, 2*time.Second, func() bool {
		return snapshotProducer(t, mgr, sess.ID).encodeHead >= 3
	}, "encode head to cover tail from mid-file start")

	engine.LastJob().Complete()
	pollUntil(t, 2*time.Second, func() bool {
		s := snapshotProducer(t, mgr, sess.ID)
		return s.exists && s.suspended
	}, "producer to park when prefix segment 0 is missing")
	if got := sessionState(t, mgr, sess.ID); got == StateCompleted {
		t.Fatalf("must not finalize session while segment 0 is missing, state=%s", got)
	}

	f, err := mgr.OpenSegment(context.Background(), in.Username, sess.ID, "00000.m4s")
	if err != nil {
		t.Fatalf("segment 0 on parked mid-file generation: %v", err)
	}
	f.Close()
	waitForStarts(t, engine, 2)
	if idx, _ := engine.LastStart(); idx != 0 {
		t.Fatalf("seek-back restart index = %d, want 0", idx)
	}
}

// A generation that started at 0 and produced the full timeline still finalizes.
func TestManagerCompletesWhenStartedFromZero(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "from-zero", "/v.mp4")
	in.StartSec = 0
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	genDir := sessionJobDir(t, mgr, sess.ID)
	writeProducedRange(t, genDir, 0, 3, true)
	pollUntil(t, 2*time.Second, func() bool {
		return snapshotProducer(t, mgr, sess.ID).encodeHead >= 3
	}, "encode head at EOF")

	engine.LastJob().Complete()
	pollUntil(t, 2*time.Second, func() bool {
		return sessionState(t, mgr, sess.ID) == StateCompleted
	}, "session completion from start")
	if snapshotProducer(t, mgr, sess.ID).exists {
		t.Fatal("producer must be dropped after full completion from index 0")
	}
}

// A tail job that reaches EOF must not finalize a generation with holes behind
// it: segments 0 and 2 exist but 1 was never produced.
func TestManagerDoesNotCompleteGenerationWithHoles(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	in := testInput(t, "holes", "/v.mp4")
	in.StartSec = 8
	sess := mustStart(t, mgr, in)
	waitForStarts(t, engine, 1)
	genDir := sessionJobDir(t, mgr, sess.ID)
	waitForPlaylistFile(t, genDir)
	writeProducedRange(t, genDir, 0, 1, false)
	writeServeableSegment(t, filepath.Join(genDir, "seg"), "00002.m4s")

	engine.LastJob().Complete()
	pollUntil(t, 2*time.Second, func() bool { s := snapshotProducer(t, mgr, sess.ID); return s.exists && s.suspended }, "producer to park with a hole")
	if _, err := os.Stat(filepath.Join(genDir, vodCompleteMarker)); err == nil {
		t.Fatal("a generation with a missing segment must not be marked complete")
	}
	if head := snapshotProducer(t, mgr, sess.ID).encodeHead; head != 1 {
		t.Fatalf("encodeHead = %d, want first hole 1", head)
	}
}
