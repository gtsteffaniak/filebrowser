package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func TestParseStreamByteRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		header    string
		size      int64
		wantStart int64
		wantEnd   int64
		wantErr   bool
	}{
		{name: "closed range", header: "bytes=0-99", size: 1000, wantStart: 0, wantEnd: 99},
		{name: "open ended", header: "bytes=10-", size: 100, wantStart: 10, wantEnd: 99},
		{name: "suffix", header: "bytes=-50", size: 100, wantStart: 50, wantEnd: 99},
		{name: "missing prefix", header: "0-99", size: 100, wantErr: true},
		{name: "multipart", header: "bytes=0-1,2-3", size: 100, wantErr: true},
		{name: "start past end", header: "bytes=100-", size: 100, wantErr: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start, end, err := parseStreamByteRange(tc.header, tc.size)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseStreamByteRange: %v", err)
			}
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("got %d-%d, want %d-%d", start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

func TestCapStreamByteRange(t *testing.T) {
	t.Parallel()
	start, end := capStreamByteRange(0, maxStreamRangeBytes)
	if end-start+1 != maxStreamRangeBytes {
		t.Fatalf("expected capped range of %d bytes, got %d", maxStreamRangeBytes, end-start+1)
	}
	start, end = capStreamByteRange(100, 150)
	if start != 100 || end != 150 {
		t.Fatalf("unexpected cap for small range: %d-%d", start, end)
	}
}

func TestStreamUseRangeOnly(t *testing.T) {
	t.Parallel()
	mediaCtx := &requestContext{user: &users.User{FrontendUser: users.FrontendUser{Permissions: users.Permissions{Download: true}}}}
	if !streamUseRangeOnly(mediaCtx, "clip.mp4") {
		t.Fatal("expected range-only for video")
	}
	if streamUseRangeOnly(mediaCtx, "notes.txt") {
		t.Fatal("did not expect range-only for text when download allowed")
	}

	noDownload := &requestContext{user: &users.User{FrontendUser: users.FrontendUser{Permissions: users.Permissions{Download: false}}}}
	if !streamUseRangeOnly(noDownload, "notes.txt") {
		t.Fatal("expected range-only without download permission")
	}

	shareCtx := &requestContext{
		user: &users.User{FrontendUser: users.FrontendUser{Permissions: users.Permissions{Download: true}}},
		share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "abc"},
			ShareSettings: share.ShareSettings{
				FrontendShareInfo: share.FrontendShareInfo{DisableDownload: true},
			},
		},
	}
	if !streamUseRangeOnly(shareCtx, "notes.txt") {
		t.Fatal("expected range-only for share with downloads disabled")
	}
}

func TestServeStreamByteRangeRejectsFullGET(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("a", 256)
	file, err := os.CreateTemp("", "stream-range-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	if _, err = file.WriteString(body); err != nil {
		t.Fatal(err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	rec := httptest.NewRecorder()
	status, err := serveStreamByteRange(rec, req, file, "clip.mp4", int64(len(body)))
	if status != http.StatusRequestedRangeNotSatisfiable || err == nil {
		t.Fatalf("expected 416, got status=%d err=%v", status, err)
	}
}

func TestServeStreamByteRangeReturnsPartialContent(t *testing.T) {
	t.Parallel()
	body := strings.Repeat("b", 512)
	file, err := os.CreateTemp("", "stream-range-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	if _, err = file.WriteString(body); err != nil {
		t.Fatal(err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Range", "bytes=0-99")
	rec := httptest.NewRecorder()
	status, err := serveStreamByteRange(rec, req, file, "clip.mp4", int64(len(body)))
	if err != nil {
		t.Fatalf("serveStreamByteRange: %v", err)
	}
	if status != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", status)
	}
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("recorder code: %d", rec.Code)
	}
	if got := rec.Body.Len(); got != 100 {
		t.Fatalf("expected 100 bytes body, got %d", got)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Range"), "bytes 0-99/512") {
		t.Fatalf("unexpected Content-Range: %q", rec.Header().Get("Content-Range"))
	}
}

func TestServeStreamByteRangeCapsOpenEndedRange(t *testing.T) {
	t.Parallel()
	size := int(maxStreamRangeBytes + 1024)
	body := strings.Repeat("c", size)
	file, err := os.CreateTemp("", "stream-range-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	if _, err = file.WriteString(body); err != nil {
		t.Fatal(err)
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	req.Header.Set("Range", "bytes=0-")
	rec := httptest.NewRecorder()
	status, err := serveStreamByteRange(rec, req, file, "song.mp3", int64(len(body)))
	if err != nil {
		t.Fatalf("serveStreamByteRange: %v", err)
	}
	if status != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", status)
	}
	if rec.Body.Len() != int(maxStreamRangeBytes) {
		t.Fatalf("expected capped body %d, got %d", maxStreamRangeBytes, rec.Body.Len())
	}
}

func TestIsMediaStreamFile(t *testing.T) {
	t.Parallel()
	if !isMediaStreamFile("movie.mp4") || !isMediaStreamFile("track.flac") || !isMediaStreamFile("episode.mkv") {
		t.Fatal("expected media extensions to match")
	}
	if isMediaStreamFile("readme.txt") {
		t.Fatal("did not expect text file to match")
	}
}
