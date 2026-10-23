package transcoding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

type producer struct {
	generation string
	jobDir     string
	plan       hlsPlan
	job        VODJob
	cancel     context.CancelFunc
	// startIndex is the first segment index this generation produces; startSec
	// is the playlist-time anchor. Segments before it are never produced until
	// a restart repositions the job.
	startIndex int
	startSec   float64
	// refs counts the sessions attached to this generation. The job is only
	// cancelled once the last attached session goes away.
	refs int
	// encodeHead is the first not-yet-ready segment index at or after
	// startIndex — the metric HLSVODJobView.EncodeHead carries.
	encodeHead int
	// jobStartedAt is when the current job launched. A repositioned job gets
	// vodRepositionCooldown to produce at its position before another
	// session's far request may move it again.
	jobStartedAt time.Time
	// jobStarting is true while runProducerJob is launching ffmpeg outside m.mu,
	// so concurrent polls wait on the in-flight launch instead of re-restarting.
	jobStarting bool
	// lastDemandAt records the latest segment/resume demand so pause-suspension
	// does not kill a producer while a seek or paused scrub is waiting on it.
	lastDemandAt time.Time
	// vodRestartIndex >= 0 means the job was cancelled to reposition; runProducer retries.
	vodRestartIndex int
	// clearSegmentsOnStart makes the next job unlink segments >= startIndex
	// before writing, per the HLSVODSession restart contract.
	clearSegmentsOnStart bool
	// vodSession owns the serve-path read leases for this generation's output
	// dir so an in-flight segment read survives a producer rewrite.
	vodSession *ffmpeg.HLSVODSession
	// suspended parks the producer: the current job was cancelled because
	// every client stayed paused past producerPauseGrace. runProducer waits
	// on resume and relaunches the job on demand. Cleared at job launch.
	suspended bool
	// resume wakes a suspended producer (segment request or un-pause
	// heartbeat). Buffered 1 so multiple wake-ups coalesce.
	resume chan struct{}
}

// clientLease tracks one heartbeat stream's freshness and pause state.
type clientLease struct {
	seen   time.Time
	paused bool
}

type playbackSession struct {
	info          SessionInfo
	reuseKey      string
	generation    string
	jobDir        string
	realPath      string
	plan          hlsPlan
	state         SessionState
	clients       map[string]clientLease
	lastActivity  time.Time
	lastHeartbeat time.Time
	playheadSec   float64
	paused        bool
	// pausedSince is when the session's live clients first became all-paused;
	// zero while any client reports playing. Producer suspension uses it.
	pausedSince     time.Time
	startedAt       time.Time
	startupDeadline time.Time
	// startSec is the playlist-time anchor of this generation's first segment.
	startSec float64
	// startIndex is the first segment index produced by this generation. When it
	// is 0 the generation covers the whole file, so any playhead is servable.
	startIndex int
	// vodSession leases open segment paths; nil until the session commits to a
	// live producer (completed-generation viewers never see rewrites).
	vodSession *ffmpeg.HLSVODSession
}

type inflightStart struct {
	done chan struct{}
	info SessionInfo
	err  error
}

// pendingStart carries a start request's identity through the start pipeline:
// admission → inflight slot → covering-generation attach → session creation.
type pendingStart struct {
	in        StartInput
	sessionID string
	reuseKey  string
	clientID  string
	flight    *inflightStart
	plan      hlsPlan
}

// Manager owns playback sessions, admission, and disk cache producers.
type Manager struct {
	engine  Engine
	store   *Store
	baseURL string

	mu          sync.Mutex
	sessions    map[string]*playbackSession
	byReuse     map[string]string
	inflight    map[string]*inflightStart
	producers   map[string]*producer
	userCounts  map[string]int
	globalCount int

	subMu   sync.Mutex
	subs    map[chan EventPayload]struct{}
	nextEvt uint64

	closed     atomic.Bool
	wg         sync.WaitGroup
	producerWg sync.WaitGroup
	runCtx     context.Context
	cancel     context.CancelFunc
	lastEvict  time.Time
}

// defaultManager is the process-wide manager used by the HTTP layer. It is stored
// atomically because request handlers read it on every transcode call while
// startup (and tests) replace it wholesale.
var defaultManager atomic.Pointer[Manager]

// SetDefault registers the process-wide transcode manager. It is called from
// app.WireServices with the manager it constructs, or with nil when ffmpeg is
// unavailable so the web layer reports transcoding as disabled.
func SetDefault(m *Manager) {
	defaultManager.Store(m)
}

// Default returns the process-wide transcode manager, or nil when transcoding is
// unavailable (ffmpeg missing or disabled).
func Default() *Manager {
	return defaultManager.Load()
}

// NewManager constructs a session manager around an Engine and a Store.
// baseURL is the public URL prefix the app is served under; segment/playlist
// URLs the manager writes are prefixed with it.
func NewManager(ctx context.Context, engine Engine, store *Store, baseURL string) (*Manager, error) {
	if engine == nil {
		engine = NewMediaEngine()
	}
	if store == nil {
		var err error
		store, err = NewStore(settings.TranscodeCacheDir())
		if err != nil {
			return nil, err
		}
	}
	if err := store.ReconcileStartup(); err != nil {
		logger.Warningf("transcode cache reconcile: %v", err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	m := &Manager{
		engine:     engine,
		store:      store,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		sessions:   make(map[string]*playbackSession),
		byReuse:    make(map[string]string),
		inflight:   make(map[string]*inflightStart),
		producers:  make(map[string]*producer),
		userCounts: make(map[string]int),
		subs:       make(map[chan EventPayload]struct{}),
		runCtx:     runCtx,
		cancel:     cancel,
	}
	m.wg.Add(1)
	go m.janitor(runCtx)
	return m, nil
}

func (m *Manager) Enabled() bool {
	return m.engine.Available()
}

func (m *Manager) Close(ctx context.Context) error {
	if !m.closed.CompareAndSwap(false, true) {
		return nil
	}
	m.cancel()

	m.mu.Lock()
	for _, p := range m.producers {
		if p.job != nil {
			p.job.Cancel()
		}
		if p.cancel != nil {
			p.cancel()
		}
	}
	for id, sess := range m.sessions {
		m.releaseSessionLocked(id, sess)
	}
	m.producers = make(map[string]*producer)
	m.mu.Unlock()

	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		m.producerWg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

type StartInput struct {
	Username         string
	UserLimit        int
	Source           string
	Path             string
	RealPath         string
	FileName         string
	Profile          Profile
	ReplaceSessionID string
	ClientID         string
	StartSec         float64
}

func (m *Manager) Start(ctx context.Context, in StartInput) (SessionInfo, error) {
	if !m.Enabled() {
		return SessionInfo{}, ErrDisabled
	}
	if m.closed.Load() {
		return SessionInfo{}, ErrDisabled
	}
	reuseKey := fmt.Sprintf("%s:%s:%s:%s", in.Username, in.Source, in.Path, in.Profile)
	clientID := normalizeClientID(in.ClientID)

	for {
		m.mu.Lock()
		if info, ok, err := m.tryReuseSessionLocked(in, reuseKey, clientID); ok || err != nil {
			// A replaced session must still be stopped even when this request
			// attaches to an existing generation, otherwise the old profile's
			// session (and its producer slot) leaks.
			if ok {
				if in.ReplaceSessionID != "" && in.ReplaceSessionID != info.ID {
					if stopErr := m.stopSessionLocked(in.Username, in.ReplaceSessionID); stopErr != nil && !errors.Is(stopErr, ErrNotFound) {
						m.mu.Unlock()
						return SessionInfo{}, stopErr
					}
				}
				// Profile/mode switches reuse a completed generation for the new
				// profile while another same-file session (old profile) is still
				// counted against the user limit — release those here.
				m.releaseSameFileClientSessionsLocked(in.Username, in.Source, in.Path, clientID, info.ID)
			}
			m.mu.Unlock()
			return info, err
		}
		if flight := m.inflight[reuseKey]; flight != nil {
			m.mu.Unlock()
			select {
			case <-flight.done:
			case <-ctx.Done():
				return SessionInfo{}, ctx.Err()
			}
			if flight.err != nil {
				return SessionInfo{}, flight.err
			}
			continue
		}

		// An already-expired replace target (janitor expiry, producer failure) means
		// there is nothing to replace, not a failed start. The admission checks
		// below still apply in that case because no slot was released.
		replaced := false
		if in.ReplaceSessionID != "" {
			stopErr := m.stopSessionLocked(in.Username, in.ReplaceSessionID)
			if stopErr != nil && !errors.Is(stopErr, ErrNotFound) {
				m.mu.Unlock()
				return SessionInfo{}, stopErr
			}
			replaced = stopErr == nil
		}
		// Same user+client+file mode switches must hand over gracefully even when
		// the client omits replaceSessionId (race / stale UI state).
		if m.releaseSameFileClientSessionsLocked(in.Username, in.Source, in.Path, clientID, "") {
			replaced = true
		}

		userLimit := userTranscodeLimit(in.UserLimit)
		if m.userCounts[in.Username] >= userLimit && !replaced {
			m.mu.Unlock()
			return SessionInfo{}, ErrUserLimit
		}
		if m.globalCount >= settings.TranscodeMaxConcurrent() && !replaced {
			m.mu.Unlock()
			return SessionInfo{}, ErrGlobalLimit
		}

		flight := &inflightStart{done: make(chan struct{})}
		m.inflight[reuseKey] = flight
		m.userCounts[in.Username]++
		m.globalCount++
		m.mu.Unlock()

		info, err := m.commitStart(ctx, pendingStart{
			in:       in,
			reuseKey: reuseKey,
			clientID: clientID,
			flight:   flight,
		})
		if err != nil {
			return SessionInfo{}, err
		}
		return info, nil
	}
}

func (m *Manager) tryReuseSessionLocked(in StartInput, reuseKey, clientID string) (SessionInfo, bool, error) {
	existingID, ok := m.byReuse[reuseKey]
	if !ok {
		return SessionInfo{}, false, nil
	}
	sess := m.sessions[existingID]
	if sess == nil {
		delete(m.byReuse, reuseKey)
		return SessionInfo{}, false, nil
	}
	if m.sessionReusableLocked(sess, in.StartSec) {
		now := time.Now()
		sess.clients[clientID] = clientLease{seen: now, paused: true}
		sess.lastActivity = now
		sess.lastHeartbeat = now
		return m.publicSession(sess, true), true, nil
	}
	// Repositioning supersedes only the requester's own session: a different
	// client seeking elsewhere gets its own generation rather than killing
	// playback for every client attached to this one.
	if in.ReplaceSessionID == existingID || soleLiveClient(sess, clientID) {
		m.supersedeSessionLocked(existingID, sess)
	}
	return SessionInfo{}, false, nil
}

// soleLiveClient reports whether clientID is the only client on the session
// that has been heard from within the reconnect grace window.
func soleLiveClient(sess *playbackSession, clientID string) bool {
	now := time.Now()
	for id, lease := range sess.clients {
		if id != clientID && now.Sub(lease.seen) <= reconnectGrace {
			return false
		}
	}
	return true
}

func (m *Manager) commitStart(ctx context.Context, ps pendingStart) (SessionInfo, error) {
	plan, err := m.engine.BuildPlan(ctx, ps.in.RealPath, ps.in.Profile)
	if err != nil {
		m.finishInflight(ps.in.Username, ps.reuseKey, ps.flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}
	ps.plan = plan

	if ps.sessionID, err = newRandomID(); err != nil {
		m.finishInflight(ps.in.Username, ps.reuseKey, ps.flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}

	// Any requested playhead may already be covered by a live producer or a
	// completed generation left in the cache. Generations are keyed only by
	// file identity plus transcode parameters, so any user playing the same
	// file with the same profile reuses them.
	if info, attached, attachErr := m.attachCoveringGeneration(ps); attached || attachErr != nil {
		return info, attachErr
	}

	generation, err := newRandomID()
	if err != nil {
		m.finishInflight(ps.in.Username, ps.reuseKey, ps.flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}
	jobDir, err := m.store.EnsureJobDir(plan.cacheFingerprint, generation)
	if err != nil {
		m.finishInflight(ps.in.Username, ps.reuseKey, ps.flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}

	startIndex, ffmpegStartSec := vodSeekParams(plan, ps.in.StartSec)
	if err := m.writeVODPlaylist(jobDir, plan.playlistBody); err != nil {
		m.finishInflight(ps.in.Username, ps.reuseKey, ps.flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		m.abortStartLocked(ps.in.Username, ps.reuseKey, ps.flight)
		return SessionInfo{}, ErrDisabled
	}

	vodSess := ffmpeg.NewHLSVODSession(ffmpeg.HLSVODSessionOptions{
		OutDir:           jobDir,
		Decision:         vodDecisionConfig(plan),
		ExplicitCopyCuts: planUsesExplicitCopy(plan),
	})
	sess := m.newSessionLocked(ps, jobDir, generation, startIndex, ffmpegStartSec, plan.mediaDurationSec)
	sess.vodSession = vodSess
	// Register the generation before the producer starts so a concurrent start
	// for the same file attaches to it instead of transcoding again.
	m.producers[ps.sessionID] = &producer{
		generation:      generation,
		jobDir:          jobDir,
		plan:            plan,
		startIndex:      startIndex,
		startSec:        ffmpegStartSec,
		encodeHead:      startIndex,
		lastDemandAt:    time.Now(),
		vodRestartIndex: -1,
		refs:            1,
		vodSession:      vodSess,
		resume:          make(chan struct{}, 1),
	}
	info := m.finishAttachLocked(ps, sess)

	m.producerWg.Add(1)
	go m.runProducer(m.runCtx, sess)
	return info, nil
}

// abortStartLocked fails a start while holding m.mu: it releases the caller's
// admission slot and resolves the inflight slot so waiters see ErrDisabled.
func (m *Manager) abortStartLocked(username, reuseKey string, flight *inflightStart) {
	m.rollbackAdmissionLocked(username)
	m.finishInflightLocked(reuseKey, flight, SessionInfo{}, ErrDisabled)
}

// finishAttachLocked registers an attached session, resolves the inflight
// start, and emits the session_started event. Caller holds m.mu.
func (m *Manager) finishAttachLocked(ps pendingStart, sess *playbackSession) SessionInfo {
	m.sessions[ps.sessionID] = sess
	m.byReuse[ps.reuseKey] = ps.sessionID
	info := m.emitSessionEventLocked(sess, eventSessionStarted)
	m.finishInflightLocked(ps.reuseKey, ps.flight, info, nil)
	return info
}

// attachCoveringGeneration attaches a new session to a generation — a live
// producer or a completed cache entry — whose produced coverage reaches the
// requested playhead. Returns attached=false when nothing covers it.
func (m *Manager) attachCoveringGeneration(ps pendingStart) (SessionInfo, bool, error) {
	targetSec := ps.in.StartSec
	m.mu.Lock()
	if m.closed.Load() {
		m.abortStartLocked(ps.in.Username, ps.reuseKey, ps.flight)
		m.mu.Unlock()
		return SessionInfo{}, true, ErrDisabled
	}
	// Multiple sessions can reference the same generation; each unique
	// producer is considered once. A seek within a live producer's coverage
	// attaches instead of launching another ffmpeg for the same file.
	seen := make(map[*producer]struct{})
	for _, p := range m.producers {
		if p == nil || p.plan.cacheFingerprint != ps.plan.cacheFingerprint {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		if !m.producerCoversTargetLocked(p, targetSec) {
			continue
		}
		p.refs++
		sess := m.newSessionLocked(ps, p.jobDir, p.generation, p.startIndex, p.startSec, p.plan.mediaDurationSec)
		sess.vodSession = p.vodSession
		sess.state = m.generationStateLocked(p.generation)
		sess.info.State = sess.state
		m.producers[ps.sessionID] = p
		info := m.finishAttachLocked(ps, sess)
		m.mu.Unlock()
		return info, true, nil
	}
	m.mu.Unlock()

	// Completed generations always ran to EOF, so covering the target only
	// requires it to start at or before the requested playhead. A generation
	// that began mid-file cannot serve segments before its first index.
	jobDir, cov, ok := m.store.FindReusableGeneration(ps.plan.cacheFingerprint)
	if !ok || (cov.FirstIndex > 0 && vodPlaylistStartSec(ps.plan, cov.FirstIndex) > targetSec+seekBackwardEpsSec) {
		return SessionInfo{}, false, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		m.abortStartLocked(ps.in.Username, ps.reuseKey, ps.flight)
		return SessionInfo{}, true, ErrDisabled
	}
	sess := m.newSessionLocked(ps, jobDir, filepath.Base(jobDir), cov.FirstIndex, vodPlaylistStartSec(ps.plan, cov.FirstIndex), ps.plan.mediaDurationSec)
	sess.state = StateCompleted
	sess.info.State = StateCompleted
	m.store.TouchGeneration(jobDir)
	info := m.finishAttachLocked(ps, sess)
	return info, true, nil
}

// producerCoversTargetLocked reports whether a live generation can serve the
// requested playhead without repositioning the job: the target is either
// already produced (ready on disk below the job's start, or at or behind the
// encode head) or close enough ahead of it to wait out inside the forward
// gap. Anything further away would restart the shared job out from under the
// other sessions attached to it, so those viewers get their own generation.
func (m *Manager) producerCoversTargetLocked(p *producer, targetSec float64) bool {
	if targetSec < -seekBackwardEpsSec || targetSec > p.plan.mediaDurationSec+seekBackwardEpsSec {
		return false
	}
	targetIndex, _ := ffmpeg.SegmentIndexForTime(targetSec, p.plan.vodList.Starts)
	if targetIndex < p.startIndex {
		// Below the job's start the segment only exists when a previous run
		// left it ready on disk; repositioning backwards would cancel the job
		// for every attached session.
		return m.vodSegmentReadyLocked(p, targetIndex)
	}
	return targetIndex <= p.encodeHead+vodForwardGap(p.plan)
}

// vodSegmentReadyLocked reports whether a previously produced segment is
// publishable from a generation's output dir. Caller holds m.mu.
func (m *Manager) vodSegmentReadyLocked(p *producer, idx int) bool {
	if planUsesExplicitCopy(p.plan) {
		ready, err := ffmpeg.HLSVODExplicitCopySegmentReady(p.jobDir, idx, vodJobRunning(p))
		return err == nil && ready
	}
	ready, err := ffmpeg.HLSVODSegmentReady(ffmpeg.HLSVODSegmentPath(p.jobDir, idx))
	return err == nil && ready
}

// vodRepositionCooldown is how long a repositioned job gets to produce at its
// position while another session rides its window before a contested far
// request may move it again. Denying the request indefinitely would starve
// the requester for the incumbent's whole session; denying it never would let
// viewers restart the job out from under each other before any coverage is
// produced. The cooldown bounds both stall and thrash — produced segments
// survive every reposition, so spaced repositions converge.
const vodRepositionCooldown = 5 * time.Second

// vodRestartBlockedLocked reports whether repositioning the shared job right
// now would strand another attached session whose live playhead sits inside
// the job's servable window [startIndex, encodeHead+gap]. The block lapses
// once the job has run for vodRepositionCooldown at its current position.
// Caller holds m.mu.
func (m *Manager) vodRestartBlockedLocked(requester *playbackSession, p *producer) bool {
	if time.Since(p.jobStartedAt) >= vodRepositionCooldown {
		return false
	}
	gap := vodForwardGap(p.plan)
	now := time.Now()
	for _, other := range m.sessions {
		if other == requester || m.producers[other.info.ID] != p {
			continue
		}
		if now.Sub(other.lastHeartbeat) > reconnectGrace {
			continue
		}
		idx, _ := ffmpeg.SegmentIndexForTime(other.playheadSec, p.plan.vodList.Starts)
		if idx >= p.startIndex && idx <= p.encodeHead+gap {
			return true
		}
	}
	return false
}

func vodForwardGap(plan hlsPlan) int {
	return ffmpeg.HLSVODMaxWaitSegmentGap(plan.segmentSec, ffmpeg.DefaultHLSVODForwardGapSec)
}

func (m *Manager) newSessionLocked(ps pendingStart, jobDir, generation string, startIndex int, startSec, endSec float64) *playbackSession {
	now := time.Now()
	return &playbackSession{
		info: SessionInfo{
			ID:                ps.sessionID,
			Username:          ps.in.Username,
			Source:            ps.in.Source,
			Path:              ps.in.Path,
			FileName:          ps.in.FileName,
			Profile:           ps.in.Profile,
			State:             StateStarting,
			StartedAt:         now.Unix(),
			HeartbeatInterval: int(defaultHeartbeatInterval.Seconds()),
			DurationSec:       ps.plan.mediaDurationSec,
			StartSec:          ps.in.StartSec,
			EndSec:            endSec,
			DeliveryBaseURL:   m.deliveryURL(ps.sessionID),
			VodPlaylist:       true,
			Encoder:           ps.plan.encoderName,
			Accel:             ps.plan.accelName,
		},
		reuseKey:        ps.reuseKey,
		generation:      generation,
		jobDir:          jobDir,
		realPath:        ps.in.RealPath,
		plan:            ps.plan,
		state:           StateStarting,
		clients:         map[string]clientLease{ps.clientID: {seen: now, paused: true}},
		lastActivity:    now,
		lastHeartbeat:   now,
		startedAt:       now,
		startupDeadline: now.Add(startupTimeout),
		startIndex:      startIndex,
		startSec:        startSec,
	}
}

// generationStateLocked reports the furthest-along lifecycle state of the
// sessions attached to a generation.
func (m *Manager) generationStateLocked(generation string) SessionState {
	state := StateStarting
	for _, sess := range m.sessions {
		if sess.generation != generation {
			continue
		}
		if sess.state == StateRunning {
			return StateRunning
		}
		state = sess.state
	}
	return state
}

func (m *Manager) finishInflight(username, reuseKey string, flight *inflightStart, info SessionInfo, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.rollbackAdmissionLocked(username)
	}
	m.finishInflightLocked(reuseKey, flight, info, err)
}

func (m *Manager) finishInflightLocked(reuseKey string, flight *inflightStart, info SessionInfo, err error) {
	flight.info = info
	flight.err = err
	delete(m.inflight, reuseKey)
	close(flight.done)
}

func (m *Manager) rollbackAdmissionLocked(username string) {
	if m.userCounts[username] > 0 {
		m.userCounts[username]--
	}
	if m.globalCount > 0 {
		m.globalCount--
	}
}

func (m *Manager) runProducer(ctx context.Context, sess *playbackSession) {
	defer m.producerWg.Done()

	// Snapshot session fields under the lock; runProducer runs concurrently with
	// code that may read or replace these fields.
	m.mu.Lock()
	gen := sess.generation
	jobDir := sess.jobDir
	realPath := sess.realPath
	plan := sess.plan
	startIndex := sess.startIndex
	m.mu.Unlock()

	for {
		outcome := m.runProducerJob(ctx, gen, jobDir, realPath, plan, startIndex)
		switch outcome {
		case producerRestartSoftware:
			logger.Warningf("transcode generation=%s hardware decode unreliable, restarting with software decode", gen)
			plan.decodeProfile.ForceSoftware = true
			// Keep the segments the hardware-decode run already produced — they
			// are valid output. Wiping the generation made every viewer refetch
			// it and stranded the request for the segment in flight when the
			// decoder failed (the client aborted it as a 503). Resume at the
			// encode head so only the partial segment at the failure point is
			// rewritten.
			m.mu.Lock()
			if p := m.producerForGenerationLocked(gen); p != nil {
				// Refresh from disk first: the failing job may have published a
				// segment since the last tick, and resuming behind it would
				// re-encode output that is already good.
				m.refreshVODEncodeHead(p, jobDir, plan)
				if p.vodRestartIndex < 0 {
					p.vodRestartIndex = p.encodeHead
				}
				p.clearSegmentsOnStart = true
			}
			m.mu.Unlock()
		case producerRestartSeek:
			// runProducerJob picks the pending vodRestartIndex back up.
			continue
		case producerSuspended:
			if !m.waitProducerResume(ctx, gen) {
				return
			}
			// Resume where the producer left off: relaunch at the encode head
			// through the same restart path a seek uses, which also clears a
			// partial segment left at the head by the cancelled job. A pending
			// restart index (request-driven reposition) wins.
			m.mu.Lock()
			if p := m.producerForGenerationLocked(gen); p != nil && p.vodRestartIndex < 0 {
				p.vodRestartIndex = p.encodeHead
				p.clearSegmentsOnStart = true
			}
			m.mu.Unlock()
			continue
		default:
			return
		}
	}
}

// producerOutcome is how a single ffmpeg job run settled.
type producerOutcome int

const (
	// producerDone: the job finished or failed — the generation is finalized.
	producerDone producerOutcome = iota
	// producerGone: the manager was closed or the generation was detached —
	// nothing left to do.
	producerGone
	// producerRestartSoftware: sustained hardware decode failures — caller
	// should retry the generation with software decode.
	producerRestartSoftware
	// producerRestartSeek: segment handler cancelled the job to reposition.
	producerRestartSeek
	// producerSuspended: every client stayed paused past the grace window, so
	// the job was cancelled. The producer parks until demand wakes it.
	producerSuspended
)

// runProducerJob runs one ffmpeg job for the generation and watches it through
// startup and steady state. Sessions attach and detach while it runs; the
// caller decides whether the outcome ends or retries the generation. A pending
// vodRestartIndex signalled by a segment request is consumed here so the job
// launches at the repositioned index.
func (m *Manager) runProducerJob(ctx context.Context, gen, jobDir, realPath string, plan hlsPlan, startIndex int) producerOutcome {
	jobCtx, cancel := context.WithCancel(ctx)
	planCopy := plan
	var vodSess *ffmpeg.HLSVODSession
	m.mu.Lock()
	if p := m.producerForGenerationLocked(gen); p != nil {
		if p.vodRestartIndex >= 0 {
			startIndex = p.vodRestartIndex
			p.startIndex = startIndex
			p.startSec = vodPlaylistStartSec(plan, startIndex)
			p.encodeHead = startIndex
			p.vodRestartIndex = -1
		}
		// Launching a job means the producer is no longer parked.
		p.suspended = false
		p.jobStarting = true
		planCopy.vodClearFromIndex = p.clearSegmentsOnStart
		p.clearSegmentsOnStart = false
		vodSess = p.vodSession
	}
	m.mu.Unlock()
	job, err := m.engine.StartVODJob(jobCtx, realPath, jobDir, planCopy, startIndex, vodSess)
	if err != nil {
		m.mu.Lock()
		if p := m.producerForGenerationLocked(gen); p != nil {
			p.jobStarting = false
		}
		m.mu.Unlock()
		cancel()
		m.failSession(gen, err)
		return producerDone
	}

	m.mu.Lock()
	if p := m.producerForGenerationLocked(gen); p != nil {
		p.jobStarting = false
		p.job = job
		p.cancel = cancel
		p.jobStartedAt = time.Now()
	} else {
		// The session went away (or was superseded) while ffmpeg started.
		job.Cancel()
		cancel()
	}
	m.mu.Unlock()

	jobDone := make(chan error, 1)
	go func() {
		jobDone <- job.Wait()
		cancel()
	}()

	startup := time.NewTimer(startupTimeout)
	defer startup.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	started := false
	for {
		select {
		case <-ctx.Done():
			return producerGone
		case err := <-jobDone:
			m.mu.Lock()
			restartSeek := false
			suspended := false
			checkHoles := false
			if p := m.producerForGenerationLocked(gen); p != nil {
				restartSeek = p.vodRestartIndex >= 0
				suspended = p.suspended && !restartSeek
				if !restartSeek && !suspended && err == nil {
					m.refreshVODEncodeHead(p, jobDir, plan)
					if p.encodeHead < plan.vodList.Count() {
						// A bounded job stopped at an already-ready tail, or
						// ffmpeg otherwise ended without covering the whole
						// timeline. Park the producer for demand-driven resume
						// instead of finalizing an incomplete generation.
						p.suspended = true
						suspended = true
					} else {
						checkHoles = true
					}
				}
			}
			m.mu.Unlock()
			if checkHoles {
				// The head only scans forward from the job start, so a tail job
				// reaching EOF says nothing about segments produced before it.
				// Scan outside the lock and park at the first hole.
				hole := m.firstVODHole(jobDir, plan)
				m.mu.Lock()
				if p := m.producerForGenerationLocked(gen); p != nil && hole >= 0 {
					if p.vodRestartIndex >= 0 {
						restartSeek = true
					} else {
						p.suspended = true
						p.encodeHead = hole
						suspended = true
					}
				}
				m.mu.Unlock()
			}
			if restartSeek {
				return producerRestartSeek
			}
			if suspended {
				return producerSuspended
			}
			m.finishProducer(gen, err, jobDir, plan)
			return producerDone
		case <-startup.C:
			if !m.store.InitReady(jobDir) {
				job.Cancel()
				m.failSession(gen, fmt.Errorf("startup timeout"))
				return producerDone
			}
		case <-ticker.C:
			if job.VTDecodeUnreliable() && !plan.decodeProfile.ForceSoftware {
				job.Cancel()
				if waitErr := <-jobDone; waitErr != nil {
					logger.Infof("transcode generation=%s job stopped for software decode retry: %v", gen, waitErr)
				}
				return producerRestartSoftware
			}
			if !started && m.store.InitReady(jobDir) {
				started = true
				m.markRunning(gen)
			}
			m.mu.Lock()
			if p := m.producerForGenerationLocked(gen); p != nil {
				m.refreshVODEncodeHead(p, jobDir, plan)
			}
			m.mu.Unlock()
		}
	}
}

// waitProducerResume parks a producer suspended for all-paused playback until
// demand (segment request or un-pause heartbeat) wakes it. Returns false when
// the producer was detached or superseded while parked so the goroutine exits.
func (m *Manager) waitProducerResume(ctx context.Context, generation string) bool {
	m.mu.Lock()
	p := m.producerForGenerationLocked(generation)
	m.mu.Unlock()
	if p == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-p.resume:
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.producerForGenerationLocked(generation) == p
}

// resumeProducerLocked wakes a producer suspended because every client was
// paused. The send never blocks: a buffered wake-up coalesces repeat signals
// and one buffered on a running producer is simply unused.
func (m *Manager) resumeProducerLocked(p *producer) {
	if p == nil {
		return
	}
	p.lastDemandAt = time.Now()
	select {
	case p.resume <- struct{}{}:
	default:
	}
}

// suspendPausedProducersLocked cancels VOD producers whose attached clients
// are all paused past producerPauseGrace. Already-produced segments stay on
// disk; the producer relaunches on the next segment request or un-pause
// heartbeat via the existing VOD restart path. Cancelling the job is the
// cross-platform pause: it frees CPU without platform-specific signals.
func (m *Manager) suspendPausedProducersLocked(now time.Time) {
	// A producer is shared by every session attached to its generation, so
	// suspension requires every session's clients to be paused, measured from
	// the most recent moment a session became all-paused.
	suspendable := map[*producer]bool{}
	pausedSince := map[*producer]time.Time{}
	for _, sess := range m.sessions {
		p := m.producers[sess.info.ID]
		if p == nil {
			continue
		}
		if _, ok := suspendable[p]; !ok {
			suspendable[p] = true
		}
		if sess.pausedSince.IsZero() {
			suspendable[p] = false
			continue
		}
		if sess.pausedSince.After(pausedSince[p]) {
			pausedSince[p] = sess.pausedSince
		}
	}
	for p, ok := range suspendable {
		if !ok || p.suspended || !vodJobRunning(p) {
			continue
		}
		if now.Sub(pausedSince[p]) < producerPauseGrace || now.Sub(p.lastDemandAt) < producerPauseGrace {
			continue
		}
		p.suspended = true
		p.job.Cancel()
	}
}

// markRunning promotes every session attached to a generation, since attached
// sessions share the single producer for that generation.
func (m *Manager) markRunning(generation string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sess := range m.sessions {
		if sess.generation != generation || sess.state != StateStarting {
			continue
		}
		sess.state = StateRunning
		sess.info.State = StateRunning
		m.emitSessionEventLocked(sess, eventSessionRunning)
	}
}

func (m *Manager) failSession(generation string, err error) {
	logger.Infof("transcode session failed generation=%s: %v", generation, err)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropProducerLocked(generation)
	for id, sess := range m.sessions {
		if sess.generation != generation {
			continue
		}
		sess.state = StateFailed
		sess.info.State = StateFailed
		m.emitSessionEventLocked(sess, eventSessionFailed)
		m.releaseSessionLocked(id, sess)
	}
}

func (m *Manager) finishProducer(generation string, err error, jobDir string, plan hlsPlan) {
	if errors.Is(err, context.Canceled) {
		logger.Debugf("transcode producer cancelled generation=%s", generation)
	} else if err != nil {
		logger.Warningf("transcode producer exited generation=%s: %v", generation, err)
	}
	if err == nil {
		m.markVODComplete(jobDir, plan.mediaDurationSec)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dropProducerLocked(generation)
	for _, sess := range m.sessions {
		if sess.generation != generation || sess.state == StateStopping {
			continue
		}
		if err != nil {
			sess.state = StateFailed
			sess.info.State = StateFailed
			m.emitSessionEventLocked(sess, eventSessionFailed)
			continue
		}
		sess.state = StateCompleted
		sess.info.State = StateCompleted
		// The generation ran to EOF, so coverage is the full media duration.
		sess.info.EndSec = sess.plan.mediaDurationSec
	}
}

// dropProducerLocked removes every producer registration for a generation once
// the job is finished, so later requests attach to the completed generation on
// disk instead of a producer that no longer runs.
func (m *Manager) dropProducerLocked(generation string) {
	for id, p := range m.producers {
		if p.generation == generation {
			delete(m.producers, id)
		}
	}
}

// producerForGenerationLocked finds the live producer for a generation. It is
// keyed under every attached session, so the originating session's departure
// must not orphan the goroutine while other sessions still share it. Caller
// holds m.mu.
func (m *Manager) producerForGenerationLocked(generation string) *producer {
	for _, p := range m.producers {
		if p.generation == generation {
			return p
		}
	}
	return nil
}

func (m *Manager) Stop(username, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopSessionLocked(username, sessionID)
}

// ownedSessionLocked returns the session when it exists and belongs to
// username. Caller holds m.mu.
func (m *Manager) ownedSessionLocked(username, sessionID string) (*playbackSession, error) {
	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrNotFound
	}
	if sess.info.Username != username {
		return nil, ErrNotOwner
	}
	return sess, nil
}

func (m *Manager) stopSessionLocked(username, sessionID string) error {
	sess, err := m.ownedSessionLocked(username, sessionID)
	if err != nil {
		return err
	}
	if sess.state == StateStopping {
		return nil
	}
	m.detachProducerLocked(sessionID)
	m.releaseSessionLocked(sessionID, sess)
	return nil
}

func (m *Manager) supersedeSessionLocked(sessionID string, sess *playbackSession) {
	if sess.state == StateStopping {
		return
	}
	sess.state = StateStopping
	sess.info.State = StateStopping
	m.detachProducerLocked(sessionID)
	m.releaseSessionLocked(sessionID, sess)
}

// releaseSameFileClientSessionsLocked stops other sessions for the same
// user+source+path that this client is attached to, keeping keepSessionID.
// Used for graceful profile/mode handover when switching streams on one file.
func (m *Manager) releaseSameFileClientSessionsLocked(username, source, path, clientID, keepSessionID string) bool {
	released := false
	for id, sess := range m.sessions {
		if id == keepSessionID || sess == nil {
			continue
		}
		if sess.info.Username != username || sess.info.Source != source || sess.info.Path != path {
			continue
		}
		if clientID != "" {
			if _, ok := sess.clients[clientID]; !ok {
				continue
			}
		}
		if err := m.stopSessionLocked(username, id); err != nil {
			continue
		}
		released = true
	}
	return released
}

// detachProducerLocked drops one session's hold on a generation. The ffmpeg job
// only stops once the last attached session releases it, so users watching the
// same file keep sharing a single producer.
func (m *Manager) detachProducerLocked(sessionID string) {
	p := m.producers[sessionID]
	if p == nil {
		return
	}
	delete(m.producers, sessionID)
	if p.refs > 0 {
		p.refs--
	}
	if p.refs > 0 {
		return
	}
	if p.job != nil {
		p.job.Cancel()
	}
	if p.cancel != nil {
		p.cancel()
	}
	// Wake a producer parked in waitProducerResume so it notices the detach
	// and exits instead of waiting on resume until manager close.
	m.resumeProducerLocked(p)
}

func (m *Manager) releaseSessionLocked(sessionID string, sess *playbackSession) {
	delete(m.sessions, sessionID)
	if m.byReuse[sess.reuseKey] == sessionID {
		delete(m.byReuse, sess.reuseKey)
	}
	if m.userCounts[sess.info.Username] > 0 {
		m.userCounts[sess.info.Username]--
	}
	if m.globalCount > 0 {
		m.globalCount--
	}
	m.emitSessionEventLocked(sess, eventSessionStopped)
}

func (m *Manager) Heartbeat(username, sessionID string, req HeartbeatRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, err := m.ownedSessionLocked(username, sessionID)
	if err != nil {
		return err
	}
	clientID := normalizeClientID(req.ClientID)
	now := time.Now()
	sess.clients[clientID] = clientLease{seen: now, paused: req.Paused}
	sess.lastHeartbeat = now
	sess.lastActivity = now
	sess.playheadSec = req.PlayheadSec
	// paused is only true when every live client is paused: a single playing
	// viewer keeps the session (and its shared producer) active.
	sess.paused = true
	for _, lease := range sess.clients {
		if !lease.paused {
			sess.paused = false
			break
		}
	}
	if sess.paused {
		if sess.pausedSince.IsZero() {
			sess.pausedSince = now
		}
	} else {
		sess.pausedSince = time.Time{}
		// An un-paused client is demand: wake a suspended producer so its next
		// segment request is already in flight.
		m.resumeProducerLocked(m.producers[sessionID])
	}
	return nil
}

func (m *Manager) Snapshot(username string, adminAll bool, userLimit int) SnapshotResponse {
	m.mu.Lock()
	defer m.mu.Unlock()
	if userLimit < 1 {
		userLimit = settings.Config.UserDefaults.Account.MaxConcurrentTranscodes
	}
	if userLimit < 1 {
		userLimit = 1
	}
	resp := SnapshotResponse{
		Enabled:      m.engine.Available(),
		GlobalLimit:  settings.TranscodeMaxConcurrent(),
		UserLimit:    userLimit,
		GlobalActive: m.globalCount,
		UserActive:   m.userCounts[username],
		CanStart:     true,
		HeartbeatSec: int(defaultHeartbeatInterval.Seconds()),
	}
	if m.userCounts[username] >= resp.UserLimit {
		resp.CanStart = false
		resp.BlockReason = "user_limit"
	}
	if m.globalCount >= resp.GlobalLimit {
		resp.CanStart = false
		resp.BlockReason = "global_limit"
	}
	for _, sess := range m.sessions {
		if adminAll || sess.info.Username == username {
			info := m.publicSession(sess, false)
			resp.Sessions = append(resp.Sessions, info)
		}
	}
	return resp
}

func (m *Manager) SessionForUser(username, sessionID string) (SessionInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, err := m.ownedSessionLocked(username, sessionID)
	if err != nil {
		return SessionInfo{}, err
	}
	return m.publicSession(sess, false), nil
}

func (m *Manager) JobDir(username, sessionID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, err := m.ownedSessionLocked(username, sessionID)
	if err != nil {
		return "", err
	}
	return sess.jobDir, nil
}

// touchJobDir returns the session's job directory and marks the session active
// under a single lock acquisition — delivery handlers call it per request.
func (m *Manager) touchJobDir(username, sessionID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, err := m.ownedSessionLocked(username, sessionID)
	if err != nil {
		return "", err
	}
	sess.lastActivity = time.Now()
	return sess.jobDir, nil
}

func (m *Manager) OpenInit(username, sessionID string) (*os.File, error) {
	jobDir, err := m.touchJobDir(username, sessionID)
	if err != nil {
		return nil, err
	}
	return m.store.OpenInit(jobDir)
}

// OpenInitWait polls until init.m4s exists or maxWait elapses. hls.js requests
// init in parallel with the first media segment; returning 503 once often leaves
// MSE in a bad state unless the client retries cleanly.
func (m *Manager) OpenInitWait(ctx context.Context, username, sessionID string, maxWait time.Duration) (*os.File, error) {
	deadline := time.Now().Add(maxWait)
	for {
		f, err := m.OpenInit(username, sessionID)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, ErrNotReady) || time.Now().After(deadline) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// OpenSegment waits for the requested VOD segment per the HLSVODSession
// decision rules — serve when publishable, wait inside the forward gap, or
// reposition the job — and returns the open file holding a serve-path lease.
// The caller must Close it; the lease is released when the file closes (after
// the handler finishes copying the response).
func (m *Manager) OpenSegment(ctx context.Context, username, sessionID, name string) (io.ReadCloser, error) {
	jobDir, err := m.touchJobDir(username, sessionID)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	sess, sessErr := m.ownedSessionLocked(username, sessionID)
	m.mu.Unlock()
	if sessErr != nil {
		return nil, sessErr
	}
	segIndex, parseErr := parseSegmentIndex(name)
	if parseErr != nil {
		return nil, parseErr
	}
	waitCtx, cancel := context.WithTimeout(ctx, time.Duration(vodSegmentWaitTimeout)*time.Second)
	defer cancel()
	f, err := m.openVODSegment(waitCtx, sess, jobDir, segIndex)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			if errors.Is(waitCtx.Err(), context.DeadlineExceeded) {
				logger.Debugf("transcode segment wait timed out session=%s seg=%d after %ds",
					sessionID, segIndex, vodSegmentWaitTimeout)
			}
			return nil, ErrNotReady
		}
		return nil, err
	}
	return f, nil
}

func (m *Manager) Playlist(username, sessionID string) ([]byte, error) {
	jobDir, err := m.touchJobDir(username, sessionID)
	if err != nil {
		return nil, err
	}
	data, err := m.store.ReadServePlaylist(jobDir)
	if err != nil {
		return nil, err
	}
	return m.rewritePlaylist(sessionID, data), nil
}

// Failed reports whether a session's producer has failed, so callers can stop
// waiting on a playlist that will never appear.
func (m *Manager) Failed(username, sessionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, err := m.ownedSessionLocked(username, sessionID)
	if err != nil {
		return false
	}
	return sess.state == StateFailed
}

func (m *Manager) Subscribe() (<-chan EventPayload, func()) {
	ch := make(chan EventPayload, 8)
	m.subMu.Lock()
	m.subs[ch] = struct{}{}
	m.subMu.Unlock()
	cancel := func() {
		m.subMu.Lock()
		delete(m.subs, ch)
		close(ch)
		m.subMu.Unlock()
	}
	return ch, cancel
}

const (
	eventSessionStarted = "session_started"
	eventSessionRunning = "session_running"
	eventSessionFailed  = "session_failed"
	eventSessionStopped = "session_stopped"
)

// emitSessionEventLocked publishes a lifecycle event for sess and returns the
// public info the payload carried. Caller holds m.mu.
func (m *Manager) emitSessionEventLocked(sess *playbackSession, evtType string) SessionInfo {
	info := m.publicSession(sess, false)
	m.publish(EventPayload{ID: m.nextEventID(), Type: evtType, Session: &info})
	return info
}

func (m *Manager) publish(evt EventPayload) {
	m.subMu.Lock()
	defer m.subMu.Unlock()
	for ch := range m.subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (m *Manager) nextEventID() string {
	id := atomic.AddUint64(&m.nextEvt, 1)
	return fmt.Sprintf("%d", id)
}

func (m *Manager) deliveryURL(sessionID string) string {
	return fmt.Sprintf("%s/api/media/transcode/sessions/%s", m.baseURL, sessionID)
}

// sessionReusableLocked reports whether a session's generation can serve the
// requested playhead. Sessions on a live producer reuse only targets inside
// the producer's servable window — a far target gets its own generation
// rather than restarting the shared job. Sessions on a completed generation
// cover anything at or after its first index. Caller holds m.mu.
func (m *Manager) sessionReusableLocked(sess *playbackSession, startSec float64) bool {
	switch sess.state {
	case StateStarting, StateRunning, StateCompleted:
	default:
		return false
	}
	if startSec < -seekBackwardEpsSec || startSec > sess.plan.mediaDurationSec+seekBackwardEpsSec {
		return false
	}
	if p := m.producers[sess.info.ID]; p != nil {
		return m.producerCoversTargetLocked(p, startSec)
	}
	return sess.startIndex <= 0 || startSec >= vodPlaylistStartSec(sess.plan, sess.startIndex)-seekBackwardEpsSec
}

func (m *Manager) rewritePlaylist(sessionID string, data []byte) []byte {
	base := m.deliveryURL(sessionID)
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "seg/"):
			lines[i] = base + "/" + trimmed
		default:
			lines[i] = strings.ReplaceAll(line, `URI="init.m4s"`, `URI="`+base+`/init.m4s"`)
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

func (m *Manager) publicSession(sess *playbackSession, reused bool) SessionInfo {
	info := sess.info
	info.State = sess.state
	info.Reused = reused
	return info
}

func (m *Manager) janitor(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.expireSessions()
			m.evictCacheIfDue()
		}
	}
}

func (m *Manager) evictCacheIfDue() {
	if time.Since(m.lastEvict) < cacheEvictionInterval {
		return
	}
	m.lastEvict = time.Now()
	active := make(map[string]struct{})
	m.mu.Lock()
	for _, sess := range m.sessions {
		active[sess.jobDir] = struct{}{}
	}
	m.mu.Unlock()
	// Refresh the mtime of generations still being watched so retention is
	// measured from last activity, not from when the transcode finished.
	for jobDir := range active {
		m.store.TouchGeneration(jobDir)
	}
	if _, err := m.store.EvictInactive(
		active,
		settings.TranscodeCacheMaxSizeBytes(),
		settings.TranscodeCacheRetention(),
	); err != nil {
		logger.Warningf("transcode cache eviction: %v", err)
	}
}

func (m *Manager) expireSessions() {
	now := time.Now()
	var toStop []string
	m.mu.Lock()
	for id, sess := range m.sessions {
		if sessionExpired(sess, now) {
			toStop = append(toStop, id)
		}
	}
	for _, id := range toStop {
		if sess := m.sessions[id]; sess != nil {
			m.detachProducerLocked(id)
			m.releaseSessionLocked(id, sess)
		}
	}
	m.suspendPausedProducersLocked(now)
	m.mu.Unlock()
}

// sessionExpired reports whether a session has outlived its clients and its
// inactivity budget. As a side effect it prunes stale client claims.
func sessionExpired(sess *playbackSession, now time.Time) bool {
	if now.Before(sess.startupDeadline) && sess.state == StateStarting {
		return false
	}
	// Client leases expire individually: a client that stops heartbeating
	// (tab closed, heartbeat failing, player remounted) frees its claim
	// after reconnectGrace instead of holding it until the inactivity
	// timeouts. Without this pruning clients only ever grew, so a dead
	// session held its user-limit slot for up to pauseInactivityTimeout.
	for clientID, lease := range sess.clients {
		if now.Sub(lease.seen) > reconnectGrace {
			delete(sess.clients, clientID)
		}
	}
	// Recompute pause state after pruning: a stale playing client must not
	// keep the session (and shared producer) awake once its lease is gone.
	if len(sess.clients) > 0 {
		sess.paused = true
		for _, lease := range sess.clients {
			if !lease.paused {
				sess.paused = false
				break
			}
		}
		if sess.paused {
			if sess.pausedSince.IsZero() {
				sess.pausedSince = now
			}
		} else {
			sess.pausedSince = time.Time{}
		}
	}
	idle := now.Sub(sess.lastActivity)
	grace := now.Sub(sess.lastHeartbeat)
	if len(sess.clients) == 0 && grace > reconnectGrace && idle > reconnectGrace {
		return true
	}
	limit := playingInactivityTimeout
	if sess.paused {
		limit = pauseInactivityTimeout
	}
	return idle > limit
}

// newRandomID returns a random hex id for sessions and generations.
func newRandomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
