package transcoding

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
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

		if in.ReplaceSessionID != "" {
			if err := m.stopSessionLocked(in.Username, in.ReplaceSessionID); err != nil {
				m.mu.Unlock()
				return SessionInfo{}, err
			}
		}

		userLimit := userTranscodeLimit(in.UserLimit)
		if m.userCounts[in.Username] >= userLimit && in.ReplaceSessionID == "" {
			m.mu.Unlock()
			return SessionInfo{}, ErrUserLimit
		}
		if m.globalCount >= settings.TranscodeMaxConcurrent() && in.ReplaceSessionID == "" {
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

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed.Load() {
		m.rollbackAdmissionLocked(in.Username)
		m.finishInflightLocked(reuseKey, flight, SessionInfo{}, ErrDisabled)
		return SessionInfo{}, ErrDisabled
	}

	now := time.Now()
	sess := &playbackSession{
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
	}

	m.sessions[sessionID] = sess
	m.byReuse[reuseKey] = sessionID
	info := m.publicSession(sess, false)
	m.finishInflightLocked(reuseKey, flight, info, nil)

	m.publish(EventPayload{ID: m.nextEventID(), Type: "session_started", Session: &info})
	m.producerWg.Add(1)
	go m.runProducer(m.runCtx, sess, in.StartSec)
	return info, nil
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

func (m *Manager) runProducer(ctx context.Context, sess *playbackSession, startSec float64) {
	defer m.producerWg.Done()

	sessionID := sess.info.ID
	gen := sess.generation
	startIndex, ffmpegStartSec := continuousSeekParams(startSec, sess.plan.segmentSec)

	jobCtx, cancel := context.WithCancel(ctx)
	job, err := m.engine.StartContinuous(jobCtx, sess.realPath, sess.jobDir, sess.plan, startIndex, ffmpegStartSec)
	if err != nil {
		cancel()
		m.failSession(sessionID, gen, err)
		return
	}

	m.mu.Lock()
	if s := m.sessions[sessionID]; s != nil && s.generation == gen {
		m.producers[sessionID] = &producer{
			sessionID:  sessionID,
			generation: gen,
			jobDir:     s.jobDir,
			plan:       s.plan,
			job:        job,
			cancel:     cancel,
		}
	} else {
		job.Cancel()
		cancel()
	}
	m.mu.Unlock()

	go func() {
		waitErr := job.Wait()
		cancel()
		m.finishProducer(sessionID, gen, waitErr)
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
			if !m.store.InitReady(sess.jobDir) {
				job.Cancel()
				m.failSession(sessionID, gen, fmt.Errorf("startup timeout"))
			}
			return
		case <-ticker.C:
			if m.store.InitReady(sess.jobDir) {
				m.markRunning(sessionID, gen)
				return
			}
		}
	}
}

func (m *Manager) markRunning(sessionID, generation string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok || sess.generation != generation {
		return
	}
	sess.state = StateRunning
	sess.info.State = StateRunning
	info := m.publicSession(sess, false)
	m.publishLocked(EventPayload{ID: m.nextEventID(), Type: "session_running", Session: &info})
}

func (m *Manager) failSession(sessionID, generation string, err error) {
	logger.Infof("transcode session failed id=%s: %v", sessionID, err)
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if !ok || sess.generation != generation {
		return
	}
	sess.state = StateFailed
	sess.info.State = StateFailed
	info := m.publicSession(sess, false)
	m.publishLocked(EventPayload{ID: m.nextEventID(), Type: "session_failed", Session: &info})
	m.releaseSessionLocked(sessionID, sess)
}

func (m *Manager) finishProducer(sessionID, generation string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess, ok := m.sessions[sessionID]
	if ok && sess.generation == generation {
		if err != nil && sess.state != StateStopping {
			sess.state = StateFailed
			sess.info.State = StateFailed
		} else if sess.state != StateStopping {
			sess.state = StateCompleted
			sess.info.State = StateCompleted
		}
	}
	if p := m.producers[sessionID]; p != nil && p.generation == generation {
		delete(m.producers, sessionID)
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
	m.cancelProducerLocked(sessionID, sess)
	m.releaseSessionLocked(sessionID, sess)
	return nil
}

func (m *Manager) supersedeSessionLocked(sessionID string, sess *playbackSession) {
	if sess.state == StateStopping {
		return
	}
	sess.state = StateStopping
	sess.info.State = StateStopping
	m.cancelProducerLocked(sessionID, sess)
	m.releaseSessionLocked(sessionID, sess)
}

func (m *Manager) cancelProducerLocked(sessionID string, sess *playbackSession) {
	if p := m.producers[sessionID]; p != nil && p.generation == sess.generation {
		if p.job != nil {
			p.job.Cancel()
		}
		if p.cancel != nil {
			p.cancel()
		}
		delete(m.producers, sessionID)
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
	data, err := m.store.ReadPlaylist(jobDir)
	if err != nil {
		return nil, err
	}
	return m.rewritePlaylist(sessionID, data), nil
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

func sessionReusable(sess *playbackSession, startSec float64) bool {
	switch sess.state {
	case StateStarting, StateRunning:
		return true
	case StateCompleted:
		return startSec <= 0
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
			m.cancelProducerLocked(id, sess)
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
