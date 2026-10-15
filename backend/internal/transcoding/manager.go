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

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

type producer struct {
	sessionID  string
	generation string
	jobDir     string
	plan       hlsPlan
	job        ContinuousJob
	cancel     context.CancelFunc
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
	// startIndex is the first segment index produced by this generation.
	startIndex int
	// coversFromStart is true when the generation's playlist starts at segment 0,
	// so any playhead is already covered by the cached segments.
	coversFromStart bool
}

type inflightStart struct {
	done chan struct{}
	info SessionInfo
	err  error
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
	producing   map[string]*producer
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

type Options struct {
	BaseURL string
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

func NewManager(ctx context.Context, engine Engine, store *Store, opts Options) (*Manager, error) {
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
		baseURL:    strings.TrimSuffix(opts.BaseURL, "/"),
		sessions:   make(map[string]*playbackSession),
		byReuse:    make(map[string]string),
		inflight:   make(map[string]*inflightStart),
		producers:  make(map[string]*producer),
		producing:  make(map[string]*producer),
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
	m.producing = make(map[string]*producer)
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
		if info, ok, err := m.tryReuseSessionLocked(reuseKey, clientID, in.StartSec); ok || err != nil {
			// A replaced session must still be stopped even when this request
			// attaches to an existing generation, otherwise the old profile's
			// session (and its producer slot) leaks.
			if ok && in.ReplaceSessionID != "" && in.ReplaceSessionID != info.ID {
				if stopErr := m.stopSessionLocked(in.Username, in.ReplaceSessionID); stopErr != nil && !errors.Is(stopErr, ErrNotFound) {
					m.mu.Unlock()
					return SessionInfo{}, stopErr
				}
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

		info, err := m.commitStart(ctx, in, reuseKey, clientID, flight)
		if err != nil {
			return SessionInfo{}, err
		}
		return info, nil
	}
}

func (m *Manager) tryReuseSessionLocked(reuseKey, clientID string, startSec float64) (SessionInfo, bool, error) {
	existingID, ok := m.byReuse[reuseKey]
	if !ok {
		return SessionInfo{}, false, nil
	}
	sess := m.sessions[existingID]
	if sess == nil {
		delete(m.byReuse, reuseKey)
		return SessionInfo{}, false, nil
	}
	if sessionReusable(sess, startSec) {
		now := time.Now()
		sess.clients[clientID] = now
		sess.lastActivity = now
		sess.lastHeartbeat = now
		return m.publicSession(sess, true), true, nil
	}
	m.supersedeSessionLocked(existingID, sess)
	return SessionInfo{}, false, nil
}

func (m *Manager) commitStart(ctx context.Context, in StartInput, reuseKey, clientID string, flight *inflightStart) (SessionInfo, error) {
	plan, err := m.engine.BuildPlan(ctx, in.RealPath, in.Profile)
	if err != nil {
		m.finishInflight(in.Username, reuseKey, flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}

	sessionID, err := newSessionID()
	if err != nil {
		m.finishInflight(in.Username, reuseKey, flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}

	// A request starting at the beginning of the file can attach to a generation
	// another session is already producing, or to a completed generation left in
	// the cache. Both are keyed only by file identity plus transcode parameters,
	// so any user playing the same file with the same profile reuses them.
	if in.StartSec <= 0 {
		if info, attached, attachErr := m.attachExistingGeneration(in, sessionID, reuseKey, clientID, flight, plan); attached || attachErr != nil {
			return info, attachErr
		}
	}

	generation, err := newSessionID()
	if err != nil {
		m.finishInflight(in.Username, reuseKey, flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}
	jobDir, err := m.store.EnsureJobDir(plan.cacheFingerprint, generation)
	if err != nil {
		m.finishInflight(in.Username, reuseKey, flight, SessionInfo{}, err)
		return SessionInfo{}, err
	}

	startIndex, ffmpegStartSec := continuousSeekParams(in.StartSec, plan.segmentSec)

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		m.rollbackAdmissionLocked(in.Username)
		m.finishInflightLocked(reuseKey, flight, SessionInfo{}, ErrDisabled)
		return SessionInfo{}, ErrDisabled
	}

	sess := m.newSessionLocked(in, sessionID, reuseKey, clientID, jobDir, generation, plan, startIndex, ffmpegStartSec)
	// Reserve the generation for this fingerprint before the producer starts so a
	// concurrent start for the same file attaches instead of transcoding again.
	producer := &producer{
		sessionID:  sessionID,
		generation: generation,
		jobDir:     jobDir,
		plan:       plan,
		refs:       1,
	}
	m.sessions[sessionID] = sess
	m.producers[sessionID] = producer
	if ffmpegStartSec <= 0 {
		m.producing[plan.cacheFingerprint] = producer
	}
	m.byReuse[reuseKey] = sessionID
	info := m.publicSession(sess, false)
	m.finishInflightLocked(reuseKey, flight, info, nil)

	m.publish(EventPayload{ID: m.nextEventID(), Type: "session_started", Session: &info})
	m.producerWg.Add(1)
	go m.runProducer(m.runCtx, sess)
	return info, nil
}

// attachExistingGeneration attaches a new session to a generation that is either
// being produced right now or already complete on disk. Returns attached=false
// when no reusable generation exists for the plan.
func (m *Manager) attachExistingGeneration(in StartInput, sessionID, reuseKey, clientID string, flight *inflightStart, plan hlsPlan) (SessionInfo, bool, error) {
	m.mu.Lock()
	if m.closed.Load() {
		m.rollbackAdmissionLocked(in.Username)
		m.finishInflightLocked(reuseKey, flight, SessionInfo{}, ErrDisabled)
		m.mu.Unlock()
		return SessionInfo{}, true, ErrDisabled
	}
	if producer := m.producing[plan.cacheFingerprint]; producer != nil {
		producer.refs++
		sess := m.newSessionLocked(in, sessionID, reuseKey, clientID, producer.jobDir, producer.generation, plan, 0, 0)
		sess.state = m.generationStateLocked(producer.generation)
		sess.info.State = sess.state
		m.sessions[sessionID] = sess
		m.producers[sessionID] = producer
		m.byReuse[reuseKey] = sessionID
		info := m.publicSession(sess, false)
		m.finishInflightLocked(reuseKey, flight, info, nil)
		m.publish(EventPayload{ID: m.nextEventID(), Type: "session_started", Session: &info})
		m.mu.Unlock()
		return info, true, nil
	}
	m.mu.Unlock()

	jobDir, ok := m.store.FindReusableGeneration(plan.cacheFingerprint)
	if !ok {
		return SessionInfo{}, false, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		m.rollbackAdmissionLocked(in.Username)
		m.finishInflightLocked(reuseKey, flight, SessionInfo{}, ErrDisabled)
		return SessionInfo{}, true, ErrDisabled
	}
	sess := m.newSessionLocked(in, sessionID, reuseKey, clientID, jobDir, filepath.Base(jobDir), plan, 0, 0)
	sess.state = StateCompleted
	sess.info.State = StateCompleted
	m.sessions[sessionID] = sess
	m.byReuse[reuseKey] = sessionID
	m.store.TouchGeneration(jobDir)
	info := m.publicSession(sess, false)
	m.finishInflightLocked(reuseKey, flight, info, nil)
	m.publish(EventPayload{ID: m.nextEventID(), Type: "session_started", Session: &info})
	return info, true, nil
}

func (m *Manager) newSessionLocked(in StartInput, sessionID, reuseKey, clientID, jobDir, generation string, plan hlsPlan, startIndex int, startSec float64) *playbackSession {
	now := time.Now()
	return &playbackSession{
		info: SessionInfo{
			ID:                sessionID,
			Username:          in.Username,
			Source:            in.Source,
			Path:              in.Path,
			FileName:          in.FileName,
			Profile:           in.Profile,
			State:             StateStarting,
			StartedAt:         now.Unix(),
			HeartbeatInterval: int(defaultHeartbeatInterval.Seconds()),
			DeliveryBaseURL:   m.deliveryURL(sessionID),
		},
		reuseKey:        reuseKey,
		generation:      generation,
		jobDir:          jobDir,
		realPath:        in.RealPath,
		plan:            plan,
		state:           StateStarting,
		clients:         map[string]time.Time{clientID: now},
		lastActivity:    now,
		lastHeartbeat:   now,
		startedAt:       now,
		startupDeadline: now.Add(startupTimeout),
		startIndex:      startIndex,
		startSec:        startSec,
		coversFromStart: startSec <= 0,
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

	jobCtx, cancel := context.WithCancel(ctx)
	job, err := m.engine.StartContinuous(jobCtx, realPath, jobDir, plan, startIndex, ffmpegStartSec)
	if err != nil {
		cancel()
		m.failSession(gen, err)
		return
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

	go func() {
		waitErr := job.Wait()
		cancel()
		m.finishProducer(gen, waitErr)
	}()

	deadline := time.NewTimer(startupTimeout)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			if !m.store.InitReady(jobDir) {
				job.Cancel()
				m.failSession(gen, fmt.Errorf("startup timeout"))
			}
			return
		case <-ticker.C:
			if m.store.InitReady(jobDir) {
				m.markRunning(gen)
				return
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
		info := m.publicSession(sess, false)
		m.publishLocked(EventPayload{ID: m.nextEventID(), Type: "session_running", Session: &info})
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
		info := m.publicSession(sess, false)
		m.publishLocked(EventPayload{ID: m.nextEventID(), Type: "session_failed", Session: &info})
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
			info := m.publicSession(sess, false)
			m.publishLocked(EventPayload{ID: m.nextEventID(), Type: "session_failed", Session: &info})
			continue
		}
		sess.state = StateCompleted
		sess.info.State = StateCompleted
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
	for fingerprint, p := range m.producing {
		if p.generation == generation {
			delete(m.producing, fingerprint)
		}
	}
}

func (m *Manager) Stop(username, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopSessionLocked(username, sessionID)
}

func (m *Manager) stopSessionLocked(username, sessionID string) error {
	sess, ok := m.sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	if sess.info.Username != username {
		return ErrNotOwner
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
	for fingerprint, other := range m.producing {
		if other == p {
			delete(m.producing, fingerprint)
		}
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
	info := m.publicSession(sess, false)
	m.publishLocked(EventPayload{ID: m.nextEventID(), Type: "session_stopped", Session: &info})
}

func (m *Manager) Heartbeat(username, sessionID string, req HeartbeatRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	if sess.info.Username != username {
		return ErrNotOwner
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
	if resp.UserLimit < 1 {
		resp.UserLimit = 1
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
	sess, ok := m.sessions[sessionID]
	if !ok {
		return SessionInfo{}, ErrNotFound
	}
	if sess.info.Username != username {
		return SessionInfo{}, ErrNotOwner
	}
	return m.publicSession(sess, false), nil
}

func (m *Manager) JobDir(username, sessionID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		return "", ErrNotFound
	}
	if sess.info.Username != username {
		return "", ErrNotOwner
	}
	return sess.jobDir, nil
}

func (m *Manager) OpenInit(username, sessionID string) (*os.File, error) {
	jobDir, err := m.JobDir(username, sessionID)
	if err != nil {
		return nil, err
	}
	m.touchActivity(sessionID)
	return m.store.OpenInit(jobDir)
}

func (m *Manager) OpenSegment(username, sessionID, name string) (*os.File, error) {
	jobDir, err := m.JobDir(username, sessionID)
	if err != nil {
		return nil, err
	}
	m.touchActivity(sessionID)
	return m.store.OpenReadySegment(jobDir, name)
}

func (m *Manager) Playlist(username, sessionID string) ([]byte, error) {
	jobDir, err := m.JobDir(username, sessionID)
	if err != nil {
		return nil, err
	}
	m.touchActivity(sessionID)
	data, ready, err := m.store.ReadServePlaylist(jobDir)
	if err != nil {
		return nil, err
	}
	if ready == 0 {
		// The playlist exists but has no aligned segments yet; report not-ready so
		// the caller keeps polling instead of serving an empty playlist.
		return nil, ErrNotReady
	}
	return m.rewritePlaylist(sessionID, data), nil
}

// Failed reports whether a session's producer has failed, so callers can stop
// waiting on a playlist that will never appear.
func (m *Manager) Failed(username, sessionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		return false
	}
	if sess.info.Username != username {
		return false
	}
	return sess.state == StateFailed
}

func (m *Manager) touchActivity(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess := m.sessions[sessionID]; sess != nil {
		sess.lastActivity = time.Now()
	}
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

func (m *Manager) publishLocked(evt EventPayload) {
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

// sessionReusable reports whether an existing generation can serve the requested
// playhead. Segments before the generation's start offset were never produced, so
// a backward seek past it must start a new generation.
func sessionReusable(sess *playbackSession, startSec float64) bool {
	switch sess.state {
	case StateStarting, StateRunning:
		return sess.startSec <= 0 || startSec >= sess.startSec
	case StateCompleted:
		// A completed generation covers the whole file when it starts at segment 0.
		return sess.coversFromStart || startSec >= sess.startSec
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
		if now.Before(sess.startupDeadline) && sess.state == StateStarting {
			continue
		}
		idle := now.Sub(sess.lastActivity)
		grace := now.Sub(sess.lastHeartbeat)
		if len(sess.clients) == 0 && grace > reconnectGrace {
			toStop = append(toStop, id)
			continue
		}
		limit := playingInactivityTimeout
		if sess.paused {
			limit = pauseInactivityTimeout
		}
		if idle > limit {
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

func newSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
