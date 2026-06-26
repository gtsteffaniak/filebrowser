package http

import (
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	streamLookaheadSec  = 45
	streamLookbackSec   = 15
	streamSeekGraceSec  = 20
	streamMetadataBytes = 8 << 20
	streamSuffixBytes   = 16 << 20
	streamWindowCookie  = "fb-sw"
	streamWindowTTL     = 2 * time.Minute
)

type streamPlaybackWindow struct {
	mu          sync.RWMutex
	durationSec float64
	currentSec  float64
	seeking     bool
	updatedAt   time.Time
}

// streamWindowUpdate is parsed from the fb-sw cookie on each range request.
type streamWindowUpdate struct {
	StreamToken string  `json:"streamToken"`
	SessionID   string  `json:"sessionId"`
	CurrentTime float64 `json:"currentTime"`
	Duration    float64 `json:"duration"`
	Seeking     bool    `json:"seeking"`
}

var streamWindowStore sync.Map // map[string]*streamPlaybackWindow

func streamWindowKey(sessionID, streamToken string) string {
	if sessionID == "" && streamToken == "" {
		return ""
	}
	if sessionID == "" {
		return streamToken
	}
	if streamToken == "" {
		return sessionID
	}
	return sessionID + "|" + streamToken
}

func streamWindowKeyFromRequest(r *http.Request) string {
	return streamWindowKey(
		r.URL.Query().Get("sessionId"),
		r.URL.Query().Get("streamToken"),
	)
}

func (w *streamPlaybackWindow) update(u streamWindowUpdate) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if u.Duration > 0 {
		w.durationSec = u.Duration
	}
	if u.CurrentTime >= 0 {
		w.currentSec = u.CurrentTime
	}
	w.seeking = u.Seeking
	w.updatedAt = time.Now()
}

func (w *streamPlaybackWindow) clipRange(start, end, fileSize int64) (allowed bool, clippedEnd int64) {
	if start < 0 || end < start || fileSize <= 0 {
		return false, 0
	}
	if end >= fileSize {
		end = fileSize - 1
	}

	// Header/index probes at file start, or tail index reads (start in last 16 MiB).
	if start < streamMetadataBytes || start >= fileSize-streamSuffixBytes {
		return true, end
	}

	w.mu.RLock()
	defer w.mu.RUnlock()

	// Mid-file reads require a fresh playback window; until the client reports
	// playhead+duration (via fb-sw cookie), deny prefetch outside header/tail zones.
	if w.updatedAt.IsZero() || time.Since(w.updatedAt) > streamWindowTTL {
		return false, 0
	}
	if w.durationSec <= 0 {
		return false, 0
	}

	centerByte := timeToByte(w.currentSec, w.durationSec, fileSize)
	lookaheadBytes := timeToByte(streamLookaheadSec, w.durationSec, fileSize)
	lookbackBytes := timeToByte(streamLookbackSec, w.durationSec, fileSize)

	minByte := centerByte - lookbackBytes
	maxByte := centerByte + lookaheadBytes
	if w.seeking {
		grace := timeToByte(streamSeekGraceSec, w.durationSec, fileSize)
		if minByte > grace {
			minByte -= grace
		} else {
			minByte = 0
		}
		maxByte += grace
	}
	if minByte < 0 {
		minByte = 0
	}
	if maxByte >= fileSize {
		maxByte = fileSize - 1
	}

	if start > maxByte {
		return false, 0
	}
	if end > maxByte {
		end = maxByte
	}
	if end < start {
		return false, 0
	}
	return true, end
}

func timeToByte(seconds, durationSec float64, fileSize int64) int64 {
	if durationSec <= 0 || fileSize <= 0 || seconds < 0 {
		return 0
	}
	ratio := seconds / durationSec
	if ratio > 1 {
		ratio = 1
	}
	return int64(ratio * float64(fileSize))
}

func streamWindowEntryFor(key string) *streamPlaybackWindow {
	if key == "" {
		return nil
	}
	if v, ok := streamWindowStore.Load(key); ok {
		return v.(*streamPlaybackWindow)
	}
	entry := &streamPlaybackWindow{}
	actual, _ := streamWindowStore.LoadOrStore(key, entry)
	return actual.(*streamPlaybackWindow)
}

func streamWindowFromCookie(r *http.Request) (streamWindowUpdate, bool) {
	c, err := r.Cookie(streamWindowCookie)
	if err != nil || c.Value == "" {
		return streamWindowUpdate{}, false
	}
	raw, err := url.QueryUnescape(c.Value)
	if err != nil {
		return streamWindowUpdate{}, false
	}
	var body streamWindowUpdate
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		return streamWindowUpdate{}, false
	}
	return body, true
}

func mergeStreamWindowFromRequest(r *http.Request) {
	token := r.URL.Query().Get("streamToken")
	sessionID := r.URL.Query().Get("sessionId")
	if token == "" {
		return
	}
	if u, ok := streamWindowFromCookie(r); ok {
		if u.StreamToken == token && (u.SessionID == "" || u.SessionID == sessionID) {
			storeStreamWindowUpdate(u)
		}
	}
}

func applyStreamPlaybackWindow(r *http.Request, start, end, fileSize int64) (allowed bool, clippedEnd int64) {
	mergeStreamWindowFromRequest(r)
	key := streamWindowKeyFromRequest(r)
	entry := streamWindowEntryFor(key)
	if entry == nil {
		return true, end
	}
	return entry.clipRange(start, end, fileSize)
}

func storeStreamWindowUpdate(u streamWindowUpdate) {
	key := streamWindowKey(u.SessionID, u.StreamToken)
	if key == "" || u.StreamToken == "" {
		return
	}
	streamWindowEntryFor(key).update(u)
}
