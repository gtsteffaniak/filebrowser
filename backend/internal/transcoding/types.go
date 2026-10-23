package transcoding

import "time"

// SessionState is the lifecycle state of a playback session.
type SessionState string

const (
	StateStarting  SessionState = "starting"
	StateRunning   SessionState = "running"
	StateStopping  SessionState = "stopping"
	StateCompleted SessionState = "completed"
	StateFailed    SessionState = "failed"
)

// Profile selects the transcode quality preset.
type Profile string

const (
	ProfileQuality   Profile = "quality"
	ProfileBalanced  Profile = "balanced"
	ProfileDataSaver Profile = "datasaver"
)

func ParseProfile(raw string) (Profile, error) {
	switch Profile(raw) {
	case ProfileQuality, ProfileBalanced, ProfileDataSaver:
		return Profile(raw), nil
	default:
		return "", ErrInvalidProfile
	}
}

const (
	defaultHeartbeatInterval = 8 * time.Second
	startupTimeout           = 90 * time.Second
	playingInactivityTimeout = 5 * time.Minute
	pauseInactivityTimeout   = 30 * time.Minute
	// reconnectGrace bounds how long a session survives once its clients stop
	// heartbeating (tab closed, viewer unmounted). It must exceed the
	// heartbeat interval with enough slack for a delayed tick.
	reconnectGrace        = 15 * time.Second
	janitorInterval       = 5 * time.Second
	cacheEvictionInterval = time.Minute
	// producerPauseGrace is how long every client on a generation must stay
	// paused before its producer is suspended. It must exceed the heartbeat
	// interval so a quick pause/unpause doesn't churn ffmpeg restarts.
	producerPauseGrace = 10 * time.Second
)

const (
	// seekBackwardEpsSec tolerates rounding when a requested playhead sits
	// just outside the servable range or on a segment boundary.
	seekBackwardEpsSec = 1.0
)

// SessionInfo is the public view of an active transcode session.
type SessionInfo struct {
	ID                string       `json:"id"`
	Username          string       `json:"username,omitempty"`
	Source            string       `json:"source"`
	Path              string       `json:"path"`
	FileName          string       `json:"fileName,omitempty"`
	Profile           Profile      `json:"profile"`
	State             SessionState `json:"state"`
	StartedAt         int64        `json:"startedAt"`
	HeartbeatInterval int          `json:"heartbeatIntervalSec"`
	DurationSec       float64      `json:"durationSec,omitempty"`
	// StartSec echoes the playhead the client requested at session start. The
	// VOD playlist is playlist-local, so clients need no offset to recover
	// source positions.
	StartSec float64 `json:"startSec,omitempty"`
	// EndSec is the end of the servable timeline (the plan's media duration).
	EndSec          float64 `json:"endSec,omitempty"`
	DeliveryBaseURL string  `json:"deliveryBaseUrl,omitempty"`
	Reused          bool    `json:"reused,omitempty"`
	// VodPlaylist marks Model A synthetic VOD playlists (playlist-local timeline).
	VodPlaylist bool `json:"vodPlaylist,omitempty"`
}

// SnapshotResponse is returned by session list/status endpoints.
type SnapshotResponse struct {
	Enabled      bool          `json:"enabled"`
	GlobalLimit  int           `json:"globalLimit"`
	UserLimit    int           `json:"userLimit"`
	GlobalActive int           `json:"globalActive"`
	UserActive   int           `json:"userActive"`
	CanStart     bool          `json:"canStart"`
	BlockReason  string        `json:"blockReason,omitempty"`
	HeartbeatSec int           `json:"heartbeatIntervalSec"`
	Sessions     []SessionInfo `json:"sessions"`
}

// StartRequest is the body for POST /api/media/transcode/sessions.
type StartRequest struct {
	Source           string  `json:"source"`
	Path             string  `json:"path"`
	ViewToken        string  `json:"viewToken"`
	Profile          string  `json:"profile"`
	ReplaceSessionID string  `json:"replaceSessionId,omitempty"`
	ClientID         string  `json:"clientId,omitempty"`
	StartSec         float64 `json:"startSec,omitempty"`
}

// HeartbeatRequest updates client activity for a session.
type HeartbeatRequest struct {
	PlayheadSec float64 `json:"playheadSec"`
	Paused      bool    `json:"paused"`
	ClientID    string  `json:"clientId,omitempty"`
}

// EventPayload is sent over SSE for capacity/lifecycle updates.
type EventPayload struct {
	ID      string           `json:"id"`
	Type    string           `json:"type"`
	Summary SnapshotResponse `json:"summary,omitempty"`
	Session *SessionInfo     `json:"session,omitempty"`
}
