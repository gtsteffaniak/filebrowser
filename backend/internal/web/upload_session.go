package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const uploadSessionHeader = "X-File-Upload-Session"

// uploadSessionTTL is how long an incomplete upload may keep exclusive claim on a path.
// Successful chunks refresh the lease; abandoned sessions expire and can be replaced.
var uploadSessionTTL = 30 * time.Minute

var errUploadSessionConflict = errors.New("conflicting upload session")

type uploadSessionEntry struct {
	id       string
	lastSeen time.Time
}

type uploadSessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]uploadSessionEntry // realPath -> active session
}

var activeUploadSessions uploadSessionRegistry

func (r *uploadSessionRegistry) acquire(realPath, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessions == nil {
		r.sessions = make(map[string]uploadSessionEntry)
	}
	now := time.Now()
	if active, ok := r.sessions[realPath]; ok {
		if now.Sub(active.lastSeen) > uploadSessionTTL {
			delete(r.sessions, realPath)
		} else if active.id != sessionID {
			return fmt.Errorf("%w for %q", errUploadSessionConflict, realPath)
		}
	}
	r.sessions[realPath] = uploadSessionEntry{id: sessionID, lastSeen: now}
	return nil
}

func (r *uploadSessionRegistry) release(realPath, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if active, ok := r.sessions[realPath]; ok && active.id == sessionID {
		delete(r.sessions, realPath)
	}
}

// expireForTest marks a path's session as expired without deleting it (for unit tests).
func (r *uploadSessionRegistry) expireForTest(realPath string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if active, ok := r.sessions[realPath]; ok {
		active.lastSeen = time.Now().Add(-uploadSessionTTL - time.Second)
		r.sessions[realPath] = active
	}
}

func isUploadSessionConflict(err error) bool {
	return errors.Is(err, errUploadSessionConflict)
}

// parseUploadSession reads X-File-Upload-Session. The value must be a short safe token.
func parseUploadSession(r *http.Request) (string, error) {
	s := strings.TrimSpace(r.Header.Get(uploadSessionHeader))
	if s == "" {
		return "", fmt.Errorf("missing %s", uploadSessionHeader)
	}
	if len(s) > 128 {
		return "", fmt.Errorf("invalid upload session")
	}
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return "", fmt.Errorf("invalid upload session")
	}
	return s, nil
}
