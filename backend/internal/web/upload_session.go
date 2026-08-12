package web

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

const uploadSessionHeader = "X-File-Upload-Session"

var errUploadSessionConflict = errors.New("conflicting upload session")

type uploadSessionRegistry struct {
	mu       sync.Mutex
	sessions map[string]string // realPath -> session ID
}

var activeUploadSessions uploadSessionRegistry

func (r *uploadSessionRegistry) acquire(realPath, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessions == nil {
		r.sessions = make(map[string]string)
	}
	active, ok := r.sessions[realPath]
	if ok && active != sessionID {
		return fmt.Errorf("%w for %q", errUploadSessionConflict, realPath)
	}
	r.sessions[realPath] = sessionID
	return nil
}

func (r *uploadSessionRegistry) release(realPath, sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if active, ok := r.sessions[realPath]; ok && active == sessionID {
		delete(r.sessions, realPath)
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
