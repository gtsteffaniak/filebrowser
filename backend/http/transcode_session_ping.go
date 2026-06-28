package http

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

type transcodeSessionPingRequest struct {
	Session   string  `json:"session"`
	SessionID string  `json:"sessionId"`
	Playhead  float64 `json:"playheadSec"`
	SeekIndex int     `json:"seekIndex"`
	Seeked    bool    `json:"seeked"`
}

// transcodeSessionPingHandler keeps HLS sessions alive and invalidates forward cache on seek.
func transcodeSessionPingHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.TranscodeEnabled() {
		return http.StatusNotFound, fmt.Errorf("transcode not enabled")
	}

	var body transcodeSessionPingRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid ping body")
	}
	sessionKey := body.Session
	if sessionKey == "" {
		sessionKey = body.SessionID
	}
	if sessionKey == "" {
		return http.StatusBadRequest, fmt.Errorf("session required")
	}

	entry, ok := activeTranscodeSessions.getHLSEntry(sessionKey, d.user.ID)
	if !ok || entry.hls == nil {
		return http.StatusNotFound, fmt.Errorf("hls session not found")
	}

	entry.hls.touchActivity(body.Playhead)

	if body.Seeked {
		fromIndex := body.SeekIndex
		if fromIndex < 0 && body.Playhead >= 0 {
			entry.hls.mu.Lock()
			starts := append([]float64(nil), entry.hls.segmentStarts...)
			entry.hls.mu.Unlock()
			fromIndex = segmentIndexForPlayhead(body.Playhead, starts)
		}
		if fromIndex >= 0 {
			entry.hls.invalidateSegmentsFrom(fromIndex)
		}
	}

	entry.hls.pruneSegmentCache()
	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent, nil
}

func segmentIndexForPlayhead(playheadSec float64, starts []float64) int {
	if playheadSec <= 0 || len(starts) == 0 {
		return 0
	}
	idx := 0
	for i, start := range starts {
		if start <= playheadSec+0.001 {
			idx = i
		}
	}
	return idx
}

func parseRuntimeSecQuery(r *http.Request) (float64, bool) {
	raw := r.URL.Query().Get("runtimeSec")
	if raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
