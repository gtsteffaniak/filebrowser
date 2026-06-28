package http

import (
	"fmt"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

const transcodePerUserLimit = 1

const (
	hlsMaxCachedSegments   = 8
	hlsSessionIdleTTL      = 5 * time.Minute
	hlsSessionPingInterval = 30 * time.Second
)

// TranscodeSession describes an active preview transcode job.
type TranscodeSession struct {
	ID            string `json:"id"`
	UserID        uint64 `json:"userId"`
	Username      string `json:"username"`
	Source        string `json:"source"`
	Path          string `json:"path"`
	FileName      string `json:"fileName"`
	StartedAt     int64  `json:"startedAt"`
	MaxResolution int    `json:"maxResolution"`
	Preset        string `json:"preset,omitempty"`
	ActiveStreams int    `json:"activeStreams,omitempty"`
}

// TranscodeSessionsResponse is returned by GET /api/media/transcode/sessions.
type TranscodeSessionsResponse struct {
	SystemLimit    int                `json:"systemLimit"`
	UserLimit      int                `json:"userLimit"`
	ActiveCount    int                `json:"activeCount"`
	CanStart       bool               `json:"canStart"`
	BlockReason    string             `json:"blockReason,omitempty"`
	TargetSource   string             `json:"targetSource,omitempty"`
	TargetPath     string             `json:"targetPath,omitempty"`
	FFmpegVersion  string             `json:"ffmpegVersion,omitempty"`
	FFmpegFeatures map[string]bool    `json:"ffmpegFeatures,omitempty"`
	Sessions       []TranscodeSession `json:"sessions"`
}

type transcodeSessionEntry struct {
	TranscodeSession
	streams int
	hls     *hlsSessionState
}

// hlsSessionState holds on-demand HLS transcode state for one file playback.
type hlsSessionState struct {
	mu               sync.Mutex
	encodeMu         sync.Mutex // serializes ffmpeg encodes for this session (avoids init/seg-0 races)
	realPath         string
	profileMode      string
	keyframeTimeline bool // true when segment cuts use probed keyframes (video-copy/remux)
	sharedInit       bool // single #EXT-X-MAP for fMP4 copy/remux sessions
	segmentCount     int
	durationSec      float64
	segmentStarts    []float64
	segmentDurations []float64
	keyframeSeekTimes []float64 // input seek hints for stream copy; playlist uses fixed grid
	segmentMediaEnds []float64  // cumulative decode timeline end after each encoded segment
	delivery         ffmpeg.HLSConfig
	params           ffmpeg.HLSSegmentParams
	init             []byte
	inits            map[int][]byte
	segments         map[int][]byte
	gopResolved      bool
	lastActivity     time.Time
	playheadSec      float64
}

type transcodeSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*transcodeSessionEntry
	byUser   map[uint64]string // userID -> active session key
}

var activeTranscodeSessions = &transcodeSessionStore{
	sessions: make(map[string]*transcodeSessionEntry),
	byUser:   make(map[uint64]string),
}

func transcodeSessionKey(userID uint64, source, path string) string {
	return fmt.Sprintf("%d:%s:%s", userID, source, path)
}

func transcodeSystemLimit() int {
	n := settings.Config.Integrations.Media.Transcode.MaxConcurrent
	if n < 1 {
		return 2
	}
	return n
}

func (s *transcodeSessionStore) snapshot() []TranscodeSession {
	out := make([]TranscodeSession, 0, len(s.sessions))
	for _, entry := range s.sessions {
		sess := entry.TranscodeSession
		sess.ActiveStreams = entry.streams
		out = append(out, sess)
	}
	return out
}

func (s *transcodeSessionStore) sessionsForUser(userID uint64) []TranscodeSession {
	var out []TranscodeSession
	for _, entry := range s.sessions {
		if entry.UserID == userID {
			sess := entry.TranscodeSession
			sess.ActiveStreams = entry.streams
			out = append(out, sess)
		}
	}
	return out
}

func (s *transcodeSessionStore) activeEntryForUser(userID uint64) *transcodeSessionEntry {
	key, ok := s.byUser[userID]
	if !ok {
		return nil
	}
	return s.sessions[key]
}

func (s *transcodeSessionStore) userHasLiveStream(userID uint64) (*transcodeSessionEntry, bool) {
	entry := s.activeEntryForUser(userID)
	if entry == nil || entry.streams <= 0 {
		return entry, false
	}
	return entry, true
}

// transcodeLimitStatus reports whether a new transcode stream may start.
// Caller must hold s.mu.
func (s *transcodeSessionStore) transcodeLimitStatus(userID uint64, source, path string) (TranscodeSessionsResponse, string) {
	systemLimit := transcodeSystemLimit()
	resp := TranscodeSessionsResponse{
		SystemLimit:  systemLimit,
		UserLimit:    transcodePerUserLimit,
		ActiveCount:  len(s.sessions),
		TargetSource: source,
		TargetPath:   path,
		Sessions:     s.snapshot(),
		CanStart:     true,
	}

	if live, blocked := s.userHasLiveStream(userID); blocked {
		if source != "" && path != "" && live.ID == transcodeSessionKey(userID, source, path) {
			return resp, ""
		}
		resp.CanStart = false
		resp.BlockReason = "user_limit"
		resp.Sessions = s.sessionsForUser(userID)
		return resp, fmt.Sprintf("user_limit active=%s streams=%d", live.ID, live.streams)
	}

	activeStreams := 0
	for _, entry := range s.sessions {
		activeStreams += entry.streams
	}
	if activeStreams >= systemLimit {
		resp.CanStart = false
		resp.BlockReason = "system_limit"
		return resp, fmt.Sprintf("system_limit activeStreams=%d limit=%d", activeStreams, systemLimit)
	}

	return resp, ""
}

func (s *transcodeSessionStore) evaluate(userID uint64, source, path string) TranscodeSessionsResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	resp, blockDetail := s.transcodeLimitStatus(userID, source, path)
	if blockDetail != "" {
		logger.Infof(
			"transcode sessions evaluate: user=%d blocked=%s target=%s:%s",
			userID, blockDetail, source, path,
		)
		return resp
	}

	logger.Debugf(
		"transcode sessions evaluate: user=%d canStart=true target=%s:%s sessions=%d",
		userID, source, path, len(s.sessions),
	)
	return resp
}

type transcodeAcquireResult struct {
	OK      bool
	Reason  string
	Session *TranscodeSession
	Status  TranscodeSessionsResponse
}

func (s *transcodeSessionStore) acquire(userID uint64, username, source, path, fileName string) transcodeAcquireResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, blockDetail := s.transcodeLimitStatus(userID, source, path)
	key := transcodeSessionKey(userID, source, path)

	if blockDetail != "" {
		reason := status.BlockReason
		logger.Infof(
			"transcode acquire rejected: user=%d reason=%s detail=%s requested=%s file=%q",
			userID, reason, blockDetail, key, fileName,
		)
		return transcodeAcquireResult{OK: false, Reason: reason, Status: status}
	}

	activeStreams := 0
	for _, entry := range s.sessions {
		activeStreams += entry.streams
	}

	tc := settings.Config.Integrations.Media.Transcode
	entry := s.sessions[key]
	if entry != nil && entry.streams > 0 {
		status.CanStart = false
		status.BlockReason = "user_limit"
		status.Sessions = s.sessionsForUser(userID)
		logger.Infof(
			"transcode acquire rejected: user=%d reason=user_limit concurrent key=%s file=%q streams=%d",
			userID, key, fileName, entry.streams,
		)
		return transcodeAcquireResult{OK: false, Reason: "user_limit", Status: status}
	}
	if entry == nil {
		entry = &transcodeSessionEntry{
			TranscodeSession: TranscodeSession{
				ID:            key,
				UserID:        userID,
				Username:      username,
				Source:        source,
				Path:          path,
				FileName:      fileName,
				StartedAt:     time.Now().Unix(),
				MaxResolution: tc.MaxResolution,
				Preset:        tc.Preset,
			},
		}
		s.sessions[key] = entry
		s.byUser[userID] = key
	}
	entry.streams++
	entry.ActiveStreams = entry.streams
	status.ActiveCount = len(s.sessions)
	status.Sessions = s.snapshot()

	logger.Infof(
		"transcode acquire ok: user=%d key=%s file=%q streams=%d totalLiveStreams=%d",
		userID, key, fileName, entry.streams, activeStreams+1,
	)

	sess := entry.TranscodeSession
	sess.ActiveStreams = entry.streams
	return transcodeAcquireResult{OK: true, Session: &sess, Status: status}
}

// acquireHLS starts or reuses an HLS transcode session without incrementing stream count on refresh.
func (s *transcodeSessionStore) acquireHLS(userID uint64, username, source, path, fileName string) transcodeAcquireResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, blockDetail := s.transcodeLimitStatus(userID, source, path)
	key := transcodeSessionKey(userID, source, path)

	if blockDetail != "" {
		reason := status.BlockReason
		return transcodeAcquireResult{OK: false, Reason: reason, Status: status}
	}

	tc := settings.Config.Integrations.Media.Transcode
	entry := s.sessions[key]
	if entry == nil {
		entry = &transcodeSessionEntry{
			TranscodeSession: TranscodeSession{
				ID:            key,
				UserID:        userID,
				Username:      username,
				Source:        source,
				Path:          path,
				FileName:      fileName,
				StartedAt:     time.Now().Unix(),
				MaxResolution: tc.MaxResolution,
				Preset:        tc.Preset,
			},
			hls: nil,
		}
		s.sessions[key] = entry
		s.byUser[userID] = key
		entry.streams = 1
	} else if entry.streams <= 0 {
		entry.streams = 1
	}

	entry.ActiveStreams = entry.streams
	status.ActiveCount = len(s.sessions)
	status.Sessions = s.snapshot()

	sess := entry.TranscodeSession
	sess.ActiveStreams = entry.streams
	return transcodeAcquireResult{OK: true, Session: &sess, Status: status}
}

func (s *transcodeSessionStore) getHLSEntry(sessionKey string, userID uint64) (*transcodeSessionEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[sessionKey]
	if !ok || entry.UserID != userID {
		return nil, false
	}
	return entry, true
}

func (s *transcodeSessionStore) releaseForUserFile(userID uint64, source, path string) {
	key := transcodeSessionKey(userID, source, path)
	s.releaseStream(key)
}

// releaseAllForUser clears every transcode session owned by userID.
func (s *transcodeSessionStore) releaseAllForUser(userID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var keys []string
	if key, ok := s.byUser[userID]; ok {
		keys = append(keys, key)
	}
	for key, entry := range s.sessions {
		if entry.UserID == userID {
			found := false
			for _, existing := range keys {
				if existing == key {
					found = true
					break
				}
			}
			if !found {
				keys = append(keys, key)
			}
		}
	}
	for _, key := range keys {
		s.releaseStreamLocked(key)
	}
}

func (s *transcodeSessionStore) releaseStream(key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseStreamLocked(key)
}

func (s *transcodeSessionStore) releaseStreamLocked(key string) {
	entry, ok := s.sessions[key]
	if !ok {
		logger.Debugf("transcode release skipped: key=%s not found", key)
		return
	}

	entry.streams--
	if entry.streams < 0 {
		entry.streams = 0
	}
	entry.ActiveStreams = entry.streams

	logger.Infof(
		"transcode stream ended: user=%d key=%s file=%q remainingStreams=%d",
		entry.UserID, key, entry.FileName, entry.streams,
	)

	if entry.streams > 0 {
		return
	}

	delete(s.sessions, key)
	if s.byUser[entry.UserID] == key {
		delete(s.byUser, entry.UserID)
	}
	logger.Infof("transcode session cleared: user=%d key=%s", entry.UserID, key)
}

func (s *transcodeSessionStore) list(userID uint64, all bool) []TranscodeSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if all {
		return s.snapshot()
	}
	return s.sessionsForUser(userID)
}

func (h *hlsSessionState) touchActivity(playheadSec float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastActivity = time.Now()
	if playheadSec >= 0 {
		h.playheadSec = playheadSec
	}
}

func (h *hlsSessionState) invalidateSegmentsFrom(fromIndex int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if fromIndex <= 0 {
		h.init = nil
		h.inits = make(map[int][]byte)
		h.segments = make(map[int][]byte)
		h.segmentMediaEnds = nil
		return
	}
	for idx := range h.segments {
		if idx >= fromIndex {
			delete(h.segments, idx)
		}
	}
	for idx := range h.inits {
		if idx >= fromIndex {
			delete(h.inits, idx)
		}
	}
	if fromIndex < len(h.segmentMediaEnds) {
		h.segmentMediaEnds = h.segmentMediaEnds[:fromIndex]
	}
}

func (h *hlsSessionState) pruneSegmentCache() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.segments) <= hlsMaxCachedSegments {
		return
	}
	playhead := h.playheadSec
	type kv struct {
		idx int
		dist float64
	}
	var items []kv
	for idx := range h.segments {
		start := float64(idx) * ffmpeg.SegmentDurationSec()
		if idx >= 0 && idx < len(h.segmentStarts) {
			start = h.segmentStarts[idx]
		}
		items = append(items, kv{idx: idx, dist: start - playhead})
	}
	// Evict segments furthest behind playhead first.
	for len(h.segments) > hlsMaxCachedSegments {
		worst := 0
		for i := 1; i < len(items); i++ {
			if items[i].dist < items[worst].dist {
				worst = i
			}
		}
		delete(h.segments, items[worst].idx)
		items = append(items[:worst], items[worst+1:]...)
	}
}

func (s *transcodeSessionStore) evictIdleSessions() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-hlsSessionIdleTTL)
	for key, entry := range s.sessions {
		if entry.hls == nil {
			continue
		}
		entry.hls.mu.Lock()
		idle := entry.hls.lastActivity.Before(cutoff)
		entry.hls.mu.Unlock()
		if idle && entry.streams <= 0 {
			delete(s.sessions, key)
			if s.byUser[entry.UserID] == key {
				delete(s.byUser, entry.UserID)
			}
			logger.Infof("transcode session evicted idle: key=%s", key)
		}
	}
}

func init() {
	go func() {
		ticker := time.NewTicker(hlsSessionPingInterval)
		defer ticker.Stop()
		for range ticker.C {
			activeTranscodeSessions.evictIdleSessions()
		}
	}()
}
