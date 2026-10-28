package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/transcoding"
)

// A path that escapes the index root must be rejected before any access-rule or
// filesystem resolution runs.
func TestTranscodeStartRejectsPathEscapingRoot(t *testing.T) {
	_, cleanup := setupTranscodeHandlerTest(t)
	defer cleanup()

	body := `{"source":"s","path":"../../etc/passwd","profile":"quality"}`
	req := httptest.NewRequest(http.MethodPost, "/api/media/transcode/sessions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	status, err := transcodeStartHandler(rec, req, &Context{})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (err=%v)", status, err)
	}
	if err == nil {
		t.Fatal("expected an error for a path escaping the index root")
	}
}

// Normalized traversal (a ".." that resolves inside the root) is accepted by the
// sanitizer, which is what lets the access check see the canonical path.
func TestTranscodeStartNormalizesInternalTraversal(t *testing.T) {
	_, cleanup := setupTranscodeHandlerTest(t)
	defer cleanup()

	// Missing view token stops the request after sanitization succeeds.
	body := `{"source":"s","path":"/public/../private/secret.mp4","profile":"quality"}`
	req := httptest.NewRequest(http.MethodPost, "/api/media/transcode/sessions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	status, err := transcodeStartHandler(rec, req, &Context{})
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 from the view-token check (err=%v)", status, err)
	}
}

func TestTranscodeStartRequiresSourceAndPath(t *testing.T) {
	_, cleanup := setupTranscodeHandlerTest(t)
	defer cleanup()

	for _, body := range []string{
		`{"source":"","path":"/a.mp4"}`,
		`{"source":"s","path":"   "}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/media/transcode/sessions", strings.NewReader(body))
		rec := httptest.NewRecorder()
		status, err := transcodeStartHandler(rec, req, &Context{})
		if status != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400 (err=%v)", body, status, err)
		}
	}
}

// transcodeErrorStatus must not map not-ready to a 2xx: hls.js treats any 2xx as
// segment bytes and would feed the error body into the demuxer.
func TestTranscodeErrorStatusNotReadyIsNotSuccess(t *testing.T) {
	if got := transcodeErrorStatus(transcoding.ErrNotReady); got < 400 {
		t.Fatalf("ErrNotReady status = %d, want a non-2xx status", got)
	}
}

func TestTranscodeErrorStatusNeedsRestartIsConflict(t *testing.T) {
	if got := transcodeErrorStatus(transcoding.ErrNeedsRestart); got != http.StatusConflict {
		t.Fatalf("ErrNeedsRestart status = %d, want 409", got)
	}
}
