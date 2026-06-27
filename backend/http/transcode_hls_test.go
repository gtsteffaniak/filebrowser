package http

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func TestHLSSegmentCount(t *testing.T) {
	t.Parallel()
	tests := []struct {
		duration float64
		want     int
	}{
		{duration: 0, want: 1},
		{duration: -1, want: 1},
		{duration: 0.5, want: 1},
		{duration: 4, want: 1},
		{duration: 4.1, want: 2},
		{duration: 120, want: 30},
	}
	for _, tc := range tests {
		tc := tc
		t.Run("", func(t *testing.T) {
			t.Parallel()
			if got := hlsSegmentCount(tc.duration); got != tc.want {
				t.Fatalf("hlsSegmentCount(%v) = %d, want %d", tc.duration, got, tc.want)
			}
		})
	}
}

func TestHLSSegmentDurationSec(t *testing.T) {
	t.Parallel()
	full := &hlsSessionState{
		segmentDurations: make([]float64, 30),
	}
	for i := range full.segmentDurations {
		full.segmentDurations[i] = 4
	}
	if got := hlsSegmentDurationSec(0, full); got != 4 {
		t.Fatalf("middle segment duration = %v, want 4", got)
	}
	if got := hlsSegmentDurationSec(29, full); got != 4 {
		t.Fatalf("last full segment duration = %v, want 4", got)
	}

	partial := &hlsSessionState{
		segmentDurations: make([]float64, 30),
	}
	for i := range partial.segmentDurations {
		partial.segmentDurations[i] = 4
	}
	partial.segmentDurations[29] = 2
	if got := hlsSegmentDurationSec(29, partial); got != 2 {
		t.Fatalf("partial last segment duration = %v, want 2", got)
	}
}

func TestHLSPlaylistURLs(t *testing.T) {
	oldBase := settings.Config.Server.BaseURL
	settings.Config.Server.BaseURL = "/testing/"
	t.Cleanup(func() {
		settings.Config.Server.BaseURL = oldBase
	})

	req := httptest.NewRequest("GET", "https://example.com/testing/api/media/transcode/hls/playlist.m3u8", nil)
	base := hlsBaseURL(req)
	if base != "https://example.com/testing" {
		t.Fatalf("unexpected base URL: %s", base)
	}
	session := "1:default:/movies/a.mkv"

	initURL := hlsInitURL(base, session, 0)
	if !strings.Contains(initURL, "/testing/api/media/transcode/hls/init.m4s?session=") {
		t.Fatalf("unexpected init URL: %s", initURL)
	}
	init1URL := hlsInitURL(base, session, 1)
	if !strings.Contains(init1URL, "/testing/api/media/transcode/hls/init/1.m4s?session=") {
		t.Fatalf("unexpected init URL for segment 1: %s", init1URL)
	}

	segURL := hlsSegURL(base, session, 3, false)
	if !strings.Contains(segURL, "/testing/api/media/transcode/hls/seg/3.m4s?session=") {
		t.Fatalf("unexpected segment URL: %s", segURL)
	}
	tsURL := hlsSegURL(base, session, 3, true)
	if !strings.Contains(tsURL, "/testing/api/media/transcode/hls/seg/3.ts?session=") {
		t.Fatalf("unexpected ts segment URL: %s", tsURL)
	}
}

func TestTranscodeSessionReleaseForUserFile(t *testing.T) {
	store := newTestSessionStore()
	setTestTranscodeMaxConcurrent(t, 2)

	acq := store.acquireHLS(1, "alice", "default", "/a.mkv", "a.mkv")
	if !acq.OK {
		t.Fatal("expected HLS acquire to succeed")
	}
	if _, blocked := store.userHasLiveStream(1); !blocked {
		t.Fatal("expected live stream after HLS acquire")
	}

	store.releaseForUserFile(1, "default", "/a.mkv")
	if _, blocked := store.userHasLiveStream(1); blocked {
		t.Fatal("expected session cleared after releaseForUserFile")
	}
}

func TestTranscodeSessionReleaseAllForUser(t *testing.T) {
	store := newTestSessionStore()
	setTestTranscodeMaxConcurrent(t, 2)

	acq := store.acquireHLS(1, "alice", "default", "/a.mkv", "a.mkv")
	if !acq.OK {
		t.Fatal("expected HLS acquire to succeed")
	}
	if _, blocked := store.userHasLiveStream(1); !blocked {
		t.Fatal("expected live stream after HLS acquire")
	}

	store.releaseAllForUser(1)
	if _, blocked := store.userHasLiveStream(1); blocked {
		t.Fatal("expected session cleared after releaseAllForUser")
	}
}

func TestTranscodeSessionReleaseForUserFileWrongPath(t *testing.T) {
	store := newTestSessionStore()
	setTestTranscodeMaxConcurrent(t, 2)

	acq := store.acquireHLS(1, "alice", "default", "/a.mkv", "a.mkv")
	if !acq.OK {
		t.Fatal("expected HLS acquire to succeed")
	}

	store.releaseForUserFile(1, "default", "/other.mkv")
	if _, blocked := store.userHasLiveStream(1); !blocked {
		t.Fatal("expected session to remain when releasing wrong path")
	}
	store.releaseForUserFile(1, "default", "/a.mkv")
}
