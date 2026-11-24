package web

import (
	"net/http"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/transcoding"
)

func TestTranscodeErrorStatus(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{transcoding.ErrNotFound, http.StatusNotFound},
		{transcoding.ErrUserLimit, http.StatusConflict},
		{transcoding.ErrGlobalLimit, http.StatusServiceUnavailable},
		{transcoding.ErrUnsupportedMedia, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		if got := transcodeErrorStatus(tc.err); got != tc.status {
			t.Fatalf("%v: got %d want %d", tc.err, got, tc.status)
		}
	}
}
