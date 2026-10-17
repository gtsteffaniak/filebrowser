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
	defaultHeartbeatInterval = 12 * time.Second
	startupTimeout           = 90 * time.Second
	playingInactivityTimeout = 5 * time.Minute
	pauseInactivityTimeout   = 30 * time.Minute
	reconnectGrace           = 30 * time.Second
	janitorInterval          = 5 * time.Second
	cacheEvictionInterval    = time.Minute
)

const (
	// repositionForwardSlackSec is how far past the produced edge a requested
	// playhead may sit before the generation is repositioned. Within slack it
	// is cheaper to let live-paced production reach the target than to restart
	// ffmpeg.
	repositionForwardSlackSec = 12.0
	// seekBackwardEpsSec tolerates rounding when a playhead sits right on a
	// generation's start boundary.
	seekBackwardEpsSec = 1.0
	// minPartialGenerationKeepSec is the minimum produced duration worth
	// caching for a generation that does not cover the whole file. Shorter
	// partial tails are discarded: they can only ever serve seeks into a tiny
	// range and cost more disk than they save.
	minPartialGenerationKeepSec = 300.0
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
	// StartSec is the media-time position of the generation's first segment —
	// the source-time offset the client adds to the 0-based HLS element
	// timeline to recover source positions.
	StartSec float64 `json:"startSec,omitempty"`
	// EndSec is the produced coverage end of the attached generation — the
	// start time of its last ready segment. For completed generations this
	// bounds the whole servable range; the client uses it to decide whether a
	// seek target can be satisfied in-session instead of repositioning.
	EndSec          float64 `json:"endSec,omitempty"`
	DeliveryBaseURL string  `json:"deliveryBaseUrl,omitempty"`
	Reused          bool    `json:"reused,omitempty"`
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
