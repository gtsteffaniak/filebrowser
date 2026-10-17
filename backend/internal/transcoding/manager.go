package transcoding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	job        ContinuousJob
	cancel     context.CancelFunc
	// startIndex is the first segment index this generation produces; startSec
	// is the segment-aligned ffmpeg -ss offset. Segments before it are never
	// produced, so coverage checks need both bounds.
	startIndex int
	startSec   float64
	// refs counts the sessions attached to this generation. The job is only
	// cancelled once the last attached session goes away.
	refs int
}

type playbackSession struct {
	info            SessionInfo
	reuseKey        string
	generation      string
	jobDir          string
	realPath        string
	plan            hlsPlan
	state           SessionState
	clients         map[string]time.Time
	lastActivity    time.Time
	lastHeartbeat   time.Time
	playheadSec     float64
	paused          bool
	startedAt       time.Time
	startupDeadline time.Time
	// startSec is the segment-aligned ffmpeg start offset for this generation.
	// Segments before it were never produced, so the session cannot serve them.
	startSec float64
	// startIndex is the first segment index produced by this generation. When it
	// is 0 the generation covers the whole file, so any playhead is servable.
	startIndex int
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
	if sessionReusable(sess, in.StartSec, m.coverageEndSecLocked(sess.plan, sess.startIndex, sess.jobDir)) {
		now := time.Now()
		sess.clients[clientID] = now
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
	for id, seen := range sess.clients {
		if id != clientID && now.Sub(seen) <= reconnectGrace {
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

	startIndex, ffmpegStartSec := continuousSeekParams(ps.in.StartSec, plan.segmentSec)

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		m.abortStartLocked(ps.in.Username, ps.reuseKey, ps.flight)
		return SessionInfo{}, ErrDisabled
	}

	sess := m.newSessionLocked(ps, jobDir, generation, startIndex, ffmpegStartSec, coverageStartSec(plan, startIndex))
	// Register the generation before the producer starts so a concurrent start
	// for the same file attaches to it instead of transcoding again.
	m.producers[ps.sessionID] = &producer{
		generation: generation,
		jobDir:     jobDir,
		plan:       plan,
		startIndex: startIndex,
		startSec:   ffmpegStartSec,
		refs:       1,
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
		sess := m.newSessionLocked(ps, p.jobDir, p.generation, p.startIndex, p.startSec, m.coverageEndSecLocked(p.plan, p.startIndex, p.jobDir))
		sess.state = m.generationStateLocked(p.generation)
		sess.info.State = sess.state
		m.producers[ps.sessionID] = p
		info := m.finishAttachLocked(ps, sess)
		m.mu.Unlock()
		return info, true, nil
	}
	m.mu.Unlock()

	// Completed generations always ran to EOF, so covering the target only
	// requires it to start at or before the requested playhead.
	jobDir, cov, ok := m.store.FindReusableGeneration(ps.plan.cacheFingerprint)
	if !ok || cov.StartSec > targetSec+seekBackwardEpsSec {
		return SessionInfo{}, false, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		m.abortStartLocked(ps.in.Username, ps.reuseKey, ps.flight)
		return SessionInfo{}, true, ErrDisabled
	}
	sess := m.newSessionLocked(ps, jobDir, filepath.Base(jobDir), cov.FirstIndex, float64(cov.FirstIndex)*ps.plan.segmentSec, cov.StartSec+cov.ProducedSec)
	sess.state = StateCompleted
	sess.info.State = StateCompleted
	m.store.TouchGeneration(jobDir)
	info := m.finishAttachLocked(ps, sess)
	return info, true, nil
}

// producerCoversTargetLocked reports whether a live generation can already
// serve the requested playhead: it must have started at or before the target
// and its produced edge must have reached it (within the forward slack that
// makes waiting cheaper than restarting ffmpeg).
func (m *Manager) producerCoversTargetLocked(p *producer, targetSec float64) bool {
	if targetSec < coverageStartSec(p.plan, p.startIndex)-seekBackwardEpsSec {
		return false
	}
	return targetSec <= m.coverageEndSecLocked(p.plan, p.startIndex, p.jobDir)+repositionForwardSlackSec
}

// coverageEndSecLocked returns the source-time end of a generation's produced
// edge — the generation start plus the produced media duration on disk, or
// the start alone when nothing has been produced yet.
func (m *Manager) coverageEndSecLocked(plan hlsPlan, startIndex int, jobDir string) float64 {
	end := coverageStartSec(plan, startIndex)
	if jobDir != "" {
		if cov := m.store.GenerationCoverage(jobDir); cov.HasPlaylist {
			// The publisher's recorded start is the real anchor — identical to
			// coverageStartSec by construction, but read from the artifact
			// rather than recomputed.
			return cov.StartSec + cov.ProducedSec
		}
	}
	return end
}

// coverageStartSec is the source-time start of a generation's first segment —
// the lower bound of the range it can serve and the offset the client adds to
// its 0-based HLS element timeline. ffmpeg anchors a stream-copy generation at
// the keyframe the seek snapped to and a transcode generation at the exact
// -ss, so positions on the planned segment grid are not usable here: emitted
// copy boundaries diverge from the plan and mid-file generations would be
// misplaced by the accumulated divergence.
func coverageStartSec(plan hlsPlan, startIndex int) float64 {
	if startIndex <= 0 {
		return 0
	}
	start := float64(startIndex) * plan.segmentSec
	if (plan.remux || plan.videoCopy) && len(plan.keyframes) > 0 {
		return ffmpeg.KeyframeSeekBefore(plan.keyframes, start)
	}
	return start
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
			StartSec:          coverageStartSec(ps.plan, startIndex),
			EndSec:            endSec,
			DeliveryBaseURL:   m.deliveryURL(ps.sessionID),
		},
		reuseKey:        ps.reuseKey,
		generation:      generation,
		jobDir:          jobDir,
		realPath:        ps.in.RealPath,
		plan:            ps.plan,
		state:           StateStarting,
		clients:         map[string]time.Time{ps.clientID: now},
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
	sessionID := sess.info.ID
	gen := sess.generation
	jobDir := sess.jobDir
	realPath := sess.realPath
	plan := sess.plan
	startIndex := sess.startIndex
	ffmpegStartSec := sess.startSec
	m.mu.Unlock()

	retried := false
	for {
		outcome := m.runProducerJob(ctx, sessionID, gen, jobDir, realPath, plan, startIndex, ffmpegStartSec)
		if outcome != producerRestartSoftware || retried {
			return
		}
		// Hardware decode proved unreliable on this file — wipe the partial
		// corrupt output and restart once with software decode. Segments are
		// re-encoded from the generation's start rather than resumed because
		// detection fires within the first seconds of output.
		retried = true
		logger.Warningf("transcode generation=%s hardware decode unreliable, restarting with software decode", gen)
		if err := m.store.ClearGenerationOutput(jobDir); err != nil {
			logger.Warningf("transcode generation=%s clearing output before decode retry: %v", gen, err)
		}
		plan.decodeProfile.ForceSoftware = true
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
)

// runProducerJob runs one ffmpeg job for the generation and watches it through
// startup and steady state. Sessions attach and detach while it runs; the
// caller decides whether the outcome ends or retries the generation.
func (m *Manager) runProducerJob(ctx context.Context, sessionID, gen, jobDir, realPath string, plan hlsPlan, startIndex int, startSec float64) producerOutcome {
	jobCtx, cancel := context.WithCancel(ctx)
	job, err := m.engine.StartContinuous(jobCtx, realPath, jobDir, plan, startIndex, startSec)
	if err != nil {
		cancel()
		m.failSession(gen, err)
		return producerDone
	}

	m.mu.Lock()
	if p := m.producers[sessionID]; p != nil && p.generation == gen {
		p.job = job
		p.cancel = cancel
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
			m.finishProducer(gen, err)
			return producerDone
		case <-startup.C:
			if !m.store.InitReady(jobDir) {
				job.Cancel()
				m.failSession(gen, fmt.Errorf("startup timeout"))
				return producerDone
			}
		case <-ticker.C:
			if job.VTDecodeUnreliable() {
				job.Cancel()
				<-jobDone
				return producerRestartSoftware
			}
			if !started && m.store.InitReady(jobDir) {
				started = true
				m.markRunning(gen)
			}
		}
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

func (m *Manager) finishProducer(generation string, err error) {
	if err != nil {
		logger.Warningf("transcode producer exited generation=%s: %v", generation, err)
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
		// The generation ran to EOF, so the produced end is final — refresh it
		// for sessions created while the producer was still live.
		sess.info.EndSec = m.coverageEndSecLocked(sess.plan, sess.startIndex, sess.jobDir)
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
	sess.clients[clientID] = now
	sess.lastHeartbeat = now
	sess.lastActivity = now
	sess.playheadSec = req.PlayheadSec
	sess.paused = req.Paused
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

func (m *Manager) OpenSegment(username, sessionID, name string) (*os.File, error) {
	jobDir, err := m.touchJobDir(username, sessionID)
	if err != nil {
		return nil, err
	}
	return m.store.OpenSegment(jobDir, name)
}

func (m *Manager) Playlist(username, sessionID string) ([]byte, error) {
	jobDir, err := m.touchJobDir(username, sessionID)
	if err != nil {
		return nil, err
	}
	data, hasSegments, err := m.store.ReadServePlaylist(jobDir)
	if err != nil {
		return nil, err
	}
	if !hasSegments {
		// The playlist exists but lists no published segments yet; report
		// not-ready so the caller keeps polling instead of serving an
		// empty playlist.
		return nil, ErrNotReady
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

// sessionReusable reports whether a generation can serve the requested
// playhead: a target before its start index or beyond the produced edge
// (live-paced, ~1x) must reposition rather than stall. coverageEndSec is the
// source-time end of the last produced segment.
func sessionReusable(sess *playbackSession, startSec, coverageEndSec float64) bool {
	switch sess.state {
	case StateStarting, StateRunning:
		if startSec < coverageStartSec(sess.plan, sess.startIndex)-seekBackwardEpsSec {
			return false
		}
		return startSec <= coverageEndSec+repositionForwardSlackSec
	case StateCompleted:
		// A completed generation ran to EOF, so it only needs to have started
		// at or before the requested playhead.
		return sess.startIndex == 0 || startSec >= coverageStartSec(sess.plan, sess.startIndex)-seekBackwardEpsSec
	default:
		return false
	}
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
	for clientID, seen := range sess.clients {
		if now.Sub(seen) > reconnectGrace {
			delete(sess.clients, clientID)
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
