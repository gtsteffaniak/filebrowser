package web

import (
	stderrors "errors"
	"net/http"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
)

func TestResolveDownloadInlineDisposition(t *testing.T) {
	t.Parallel()
	force, err := resolveDownloadInlineDisposition("readme.txt", true)
	if err != nil || !force {
		t.Fatalf("text inline: force=%v err=%v", force, err)
	}
	force, err = resolveDownloadInlineDisposition("clip.mp4", false)
	if err != nil || force {
		t.Fatalf("video download: force=%v err=%v", force, err)
	}
	_, err = resolveDownloadInlineDisposition("clip.mp4", true)
	if !stderrors.Is(err, errors.ErrUseMediaStream) {
		t.Fatalf("expected ErrUseMediaStream for inline video, got %v", err)
	}
}

func TestDownloadResponseRecordsActivity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		err    error
		want   bool
	}{
		{name: "200 ok", status: http.StatusOK, want: true},
		{name: "206 partial", status: http.StatusPartialContent, want: true},
		{name: "416 range unsatisfiable", status: http.StatusRequestedRangeNotSatisfiable, want: false},
		{name: "403 forbidden", status: http.StatusForbidden, want: false},
		{name: "error with 200", status: http.StatusOK, err: stderrors.New("write failed"), want: false},
		{name: "zero status", status: 0, want: false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := downloadResponseRecordsActivity(tc.status, tc.err); got != tc.want {
				t.Fatalf("downloadResponseRecordsActivity(%d, err=%v) = %v, want %v", tc.status, tc.err != nil, got, tc.want)
			}
		})
	}
}
