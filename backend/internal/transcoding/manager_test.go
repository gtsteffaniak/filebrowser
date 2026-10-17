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
	plan := hlsPlan{segmentSec: 4}
	running := &playbackSession{state: StateRunning, plan: plan}
	// A generation that starts at segment 0 covers the whole file, so a
	// completed session serves any playhead without transcoding again.
	completedFromStart := &playbackSession{state: StateCompleted, plan: plan}
	// A generation that started at segment 15 never produced earlier segments.
	completedPartial := &playbackSession{state: StateCompleted, startSec: 60, startIndex: 15, plan: plan}
	runningPartial := &playbackSession{state: StateRunning, startSec: 60, startIndex: 15, plan: plan}
	failed := &playbackSession{state: StateFailed, plan: plan}

	if !sessionReusable(running, 12, 120) {
		t.Fatal("running session should be reusable while playing within coverage")
	}
	if !sessionReusable(running, 130, 120) {
		t.Fatal("a target just past the produced edge should wait for paced production")
	}
	if sessionReusable(running, 300, 120) {
		t.Fatal("a target far past the produced edge must reposition the generation")
	}
	if !sessionReusable(completedFromStart, 0, 0) {
		t.Fatal("completed session should be reusable from start")
	}
	if !sessionReusable(completedFromStart, 12, 0) {
		t.Fatal("a completed generation that covers the file should serve a later seek")
	}
	if sessionReusable(completedPartial, 12, 0) {
		t.Fatal("a mid-file generation must not be reused for an earlier playhead")
	}
	if !sessionReusable(completedPartial, 90, 0) {
		t.Fatal("a completed mid-file generation should serve a later playhead")
	}
	if sessionReusable(runningPartial, 12, 300) {
		t.Fatal("a mid-file running generation must not be reused for an earlier playhead")
	}
	if !sessionReusable(runningPartial, 90, 300) {
		t.Fatal("a mid-file running generation should serve a later playhead")
	}
	if sessionReusable(runningPartial, 90, 64) {
		t.Fatal("a playhead past the produced edge must not reuse a running generation")
	}
	if sessionReusable(failed, 0, 0) {
		t.Fatal("failed session should not be reused")
	}
}

func TestCoverageStartSec(t *testing.T) {
	// A stream-copy seek generation is anchored by ffmpeg at the keyframe the
	// input seek snapped to — not at the fixed grid position the -ss asked
	// for, and never at the planned segment timeline, which diverges from the
	// emitted boundaries and would shift the client's element timeline.
	copyPlan := hlsPlan{segmentSec: 4, videoCopy: true, keyframes: []float64{0, 10, 40, 80, 1885, 1890}}
	if got := coverageStartSec(copyPlan, 472); got != 1885 {
		t.Fatalf("copy generation start = %v, want snapped keyframe 1885", got)
	}
	if got := coverageStartSec(copyPlan, 0); got != 0 {
		t.Fatalf("generation from media start = %v, want 0", got)
	}
	// A copy plan without probed keyframes falls back to the -ss grid value.
	if got := coverageStartSec(hlsPlan{segmentSec: 4, remux: true}, 472); got != 1888 {
		t.Fatalf("copy generation without keyframes = %v, want grid 1888", got)
	}
	// Transcode output starts at the exact -ss — the segment grid position.
	if got := coverageStartSec(hlsPlan{segmentSec: 4}, 472); got != 1888 {
		t.Fatalf("transcode generation start = %v, want grid 1888", got)
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
	if producer.refs != 2 {
		t.Fatalf("producer refs = %d, want 2", producer.refs)
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
	if idx, sec := engine.LastStart(); idx != 75 || sec != 300 {
		t.Fatalf("new generation start = (%d, %v), want (75, 300)", idx, sec)
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

// A completed partial generation serves seeks into its own range but must
// never be attached as if it covered the file start.
func TestManagerAttachesCoveringCompletedGeneration(t *testing.T) {
	engine := NewFakeEngine()
	mgr, store := newTestManager(t, engine)

	realPath := filepath.Join(t.TempDir(), "v.mp4")
	plan, err := engine.BuildPlan(context.Background(), realPath, ProfileQuality)
	if err != nil {
		t.Fatal(err)
	}
	// A finished tail covering segments 10..19 (40s..80s on the 4s grid).
	writeProducedRange(t, store.JobDir(plan.cacheFingerprint, "gen-tail"), 10, 10, true)

	// A seek to 50s lands inside the tail — attach without running ffmpeg.
	in := testInput(t, "u", "/v.mp4")
	in.UserLimit = 4
	in.RealPath = realPath
	in.ClientID = "seek-in-range"
	in.StartSec = 50
	attached := mustStart(t, mgr, in)
	if attached.State != StateCompleted {
		t.Fatalf("attached session state = %q, want completed", attached.State)
	}
	if engine.StartCount() != 0 {
		t.Fatalf("a covering cache hit must not start ffmpeg, starts=%d", engine.StartCount())
	}
	sess := sessionFor(t, mgr, attached.ID)
	if sess.startIndex != 10 || sess.startSec != 40 {
		t.Fatalf("session start = (%d, %v), want (10, 40)", sess.startIndex, sess.startSec)
	}
	// The published start is the media-time origin of the generation — the
	// offset the player adds to the 0-based element timeline.
	if attached.StartSec != 40 {
		t.Fatalf("published session startSec = %v, want 40", attached.StartSec)
	}
	// Segments 10..19 produced on the 4s grid: the servable range ends at the
	// end of segment 19 (76+4) — the client needs this to tell "buffered" from
	// "servable" coverage on a completed generation.
	if attached.EndSec != 80 {
		t.Fatalf("published session endSec = %v, want 80", attached.EndSec)
	}

	// A request before the tail's coverage must not attach to it.
	fresh := in
	fresh.ClientID = "seek-out-of-range"
	fresh.StartSec = 5
	info, err := mgr.Start(context.Background(), fresh)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID == attached.ID {
		t.Fatal("a playhead before the partial generation needs its own job")
	}
	if info.StartSec != 4 {
		t.Fatalf("fresh generation startSec = %v, want 4 (segment-aligned to the 4s grid)", info.StartSec)
	}
	waitForStarts(t, engine, 1)
	if _, err := mgr.SessionForUser("u", attached.ID); err != nil {
		t.Fatalf("the first session must survive another client's seek: %v", err)
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
	// Nothing produced yet: the produced edge is still the generation start.
	if info.EndSec != 0 {
		t.Fatalf("fresh session endSec = %v, want 0", info.EndSec)
	}

	// Three emitted 4s segments (12s of produced media) with ENDLIST.
	writeProducedRange(t, sessionJobDir(t, mgr, info.ID), 0, 3, true)
	engine.job.Cancel()

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
	if got.EndSec != 12 {
		t.Fatalf("completed session endSec = %v, want 12", got.EndSec)
	}
}

// Sustained hardware decode failures must restart the generation once with
// software decode: the corrupt partial output is wiped so it cannot be served
// or counted toward coverage, and a second failure does not loop restarts.
func TestManagerRestartsProducerWithSoftwareDecode(t *testing.T) {
	engine := NewFakeEngine()
	mgr, _ := newTestManager(t, engine)

	info := mustStart(t, mgr, testInput(t, "user-a", "/movie.mp4"))
	waitForStarts(t, engine, 1)

	// Seed output resembling produced coverage from the failing run.
	jobDir := sessionJobDir(t, mgr, info.ID)
	writeProducedRange(t, jobDir, 0, 3, false)

	engine.LastJob().SetVTUnreliable()

	waitForStarts(t, engine, 2)
	if got := engine.LastPlan().decodeProfile; !got.ForceSoftware {
		t.Fatalf("retry decode profile = %+v, want ForceSoftware", got)
	}
	if _, err := os.Stat(filepath.Join(jobDir, "seg", "00000.m4s")); !os.IsNotExist(err) {
		t.Fatalf("corrupt segment should have been wiped, stat err=%v", err)
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

func (b *blockingStartEngine) StartContinuous(ctx context.Context, realPath, outputDir string, plan hlsPlan, startIndex int, startSec float64) (ContinuousJob, error) {
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
func producerFor(t *testing.T, mgr *Manager, id string) *producer {
	t.Helper()
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	return mgr.producers[id]
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

// writeProducedRange writes a generation's published output — seg/ files plus
// index.m3u8 and job.json covering [firstIdx, firstIdx+count) — so coverage
// checks see the produced edge the publisher would have recorded.
func writeProducedRange(t *testing.T, genDir string, firstIdx, count int, endList bool) {
	t.Helper()
	segDir := filepath.Join(genDir, "seg")
	if err := os.MkdirAll(segDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(genDir, "init.m4s"), make([]byte, 1024), 0o644); err != nil {
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
	lastReady := firstIdx + count - 1
	complete := endList && count > 0
	status := fmt.Sprintf(`{"startSec":%g,"firstIndex":%d,"lastReadyIndex":%d,"producedSec":%g,"endList":%t,"complete":%t}`,
		float64(firstIdx)*4, firstIdx, lastReady, float64(count)*4, endList, complete)
	if err := os.WriteFile(filepath.Join(genDir, "job.json"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}
}

// userIDKeyPattern matches a JSON key that looks like a user identifier, at any
// depth — the guard TestSessionInfoOmitsUserID applies to session payloads.
var userIDKeyPattern = regexp.MustCompile(`(?i)"user[-_ ]?id"\s*:`)
