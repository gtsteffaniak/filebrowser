package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func publicMediaShareContext(owner *users.User, sourceRoot string, editable share.ShareEditable) *Context {
	link := share.Share{
		ShareColumns: share.ShareColumns{
			Hash: "mediashare",
			Path: "/shared",
		},
		FrontendShareInfo: editable.FrontendShareInfo,
		ShareLimits:       editable.ShareLimits,
		SourcePath:        sourceRoot,
	}
	return &Context{
		User: &users.User{
			FrontendUser: users.FrontendUser{Username: users.AnonymousUserName},
		},
		ShareUser: owner,
		IndexPath: "/shared/song.mp3",
		Share:     link,
	}
}

func TestPublicLyricsHandler_BlocksWhenFileViewerDisabled(t *testing.T) {
	sourceRoot, _, owner := setupShareArchiveDownloadTest(t)
	d := publicMediaShareContext(owner, sourceRoot, share.ShareEditable{DisableFileViewer: true})
	req := httptest.NewRequest(http.MethodGet, "/public/api/media/lyrics?hash=mediashare&path=/song.mp3", nil)
	rec := httptest.NewRecorder()

	status, err := publicLyricsHandler(rec, req, d)
	if err == nil {
		t.Fatal("expected error when file viewer is disabled")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestPublicLyricsHandler_BlocksWhenDownloadLimitReached(t *testing.T) {
	sourceRoot, _, owner := setupShareArchiveDownloadTest(t)
	d := publicMediaShareContext(owner, sourceRoot, share.ShareEditable{
		DownloadsLimit: 1,
	})
	d.Share.Downloads = 1
	req := httptest.NewRequest(http.MethodGet, "/public/api/media/lyrics?hash=mediashare&path=/song.mp3", nil)
	rec := httptest.NewRecorder()

	status, err := publicLyricsHandler(rec, req, d)
	if err == nil {
		t.Fatal("expected error when download limit is exhausted")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestPublicSubtitlesHandler_BlocksWhenDownloadLimitReached(t *testing.T) {
	sourceRoot, _, owner := setupShareArchiveDownloadTest(t)
	d := publicMediaShareContext(owner, sourceRoot, share.ShareEditable{
		DownloadsLimit: 1,
	})
	d.Share.Downloads = 1
	req := httptest.NewRequest(http.MethodGet, "/public/api/media/subtitles?hash=mediashare&path=/movie.mp4&name=movie.srt&embedded=false", nil)
	rec := httptest.NewRecorder()

	status, err := publicSubtitlesHandler(rec, req, d)
	if err == nil {
		t.Fatal("expected error when download limit is exhausted")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestPublicSubtitlesHandler_BlocksWhenDownloadDisabled(t *testing.T) {
	sourceRoot, _, owner := setupShareArchiveDownloadTest(t)
	d := publicMediaShareContext(owner, sourceRoot, share.ShareEditable{DisableDownload: true})
	req := httptest.NewRequest(http.MethodGet, "/public/api/media/subtitles?hash=mediashare&path=/movie.mp4&name=movie.srt&embedded=false", nil)
	rec := httptest.NewRecorder()

	status, err := publicSubtitlesHandler(rec, req, d)
	if err == nil {
		t.Fatal("expected error when downloads are disabled")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}
