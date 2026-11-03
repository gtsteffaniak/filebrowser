package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

func TestPublicTextPreviewHandler_BlocksWhenFileViewerDisabled(t *testing.T) {
	sourceRoot, _, owner := setupShareArchiveDownloadTest(t)
	d := publicMediaShareContext(owner, sourceRoot, share.ShareEditable{DisableFileViewer: true})
	req := httptest.NewRequest(http.MethodGet, "/public/api/resources/preview/text?hash=mediashare&path=/notes.txt", nil)
	rec := httptest.NewRecorder()

	status, err := publicTextPreviewHandler(rec, req, d)
	if err == nil {
		t.Fatal("expected error when file viewer is disabled")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestPublicTextPreviewHandler_BlocksWhenDownloadLimitReached(t *testing.T) {
	sourceRoot, _, owner := setupShareArchiveDownloadTest(t)
	d := publicMediaShareContext(owner, sourceRoot, share.ShareEditable{DownloadsLimit: 1})
	d.Share.Downloads = 1
	req := httptest.NewRequest(http.MethodGet, "/public/api/resources/preview/text?hash=mediashare&path=/notes.txt", nil)
	rec := httptest.NewRecorder()

	status, err := publicTextPreviewHandler(rec, req, d)
	if err == nil {
		t.Fatal("expected error when download limit is exhausted")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}
