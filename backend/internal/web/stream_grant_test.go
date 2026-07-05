package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestMintAndValidateViewGrant(t *testing.T) {
	t.Parallel()
	d := &requestContext{
		User: &users.User{ID: 42, FrontendUser: users.FrontendUser{Username: "alice"}},
	}
	token, err := mintViewGrant(d, "default", "/docs/track.mp3")
	if err != nil {
		t.Fatalf("mintViewGrant: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if err := ValidateViewGrant(token, d, "default", "/docs/track.mp3"); err != nil {
		t.Fatalf("ValidateViewGrant: %v", err)
	}
}

func TestValidateViewGrantWrongUser(t *testing.T) {
	t.Parallel()
	owner := &requestContext{User: &users.User{ID: 1}}
	other := &requestContext{User: &users.User{ID: 2}}
	token, err := mintViewGrant(owner, "default", "/a.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateViewGrant(token, other, "default", "/a.mp3"); err == nil {
		t.Fatal("expected viewer mismatch error")
	}
}

func TestValidateViewGrantWrongPath(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 1}}
	token, err := mintViewGrant(d, "default", "/a.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateViewGrant(token, d, "default", "/b.mp3"); err == nil {
		t.Fatal("expected path mismatch error")
	}
}

func TestValidateViewGrantExpired(t *testing.T) {
	t.Parallel()
	token, err := utils.RandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	utils.ViewGrantsCache.Set(token, utils.ViewGrant{
		UserID:    1,
		Source:    "default",
		Path:      "/a.mp3",
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	})
	d := &requestContext{User: &users.User{ID: 1}}
	if err := ValidateViewGrant(token, d, "default", "/a.mp3"); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestValidateViewGrantShareBinding(t *testing.T) {
	t.Parallel()
	d := &requestContext{
		User:  &users.User{ID: 1},
		Share: share.Share{ShareColumns: share.ShareColumns{Hash: "abc123"}},
	}
	token, err := mintViewGrant(d, "srv", "/file.mp3")
	if err != nil {
		t.Fatal(err)
	}
	wrongShare := &requestContext{
		User:  &users.User{ID: 1},
		Share: share.Share{ShareColumns: share.ShareColumns{Hash: "other"}},
	}
	if err := ValidateViewGrant(token, wrongShare, "srv", "/file.mp3"); err == nil {
		t.Fatal("expected share mismatch error")
	}
}

func TestAttachViewTokenForAllFileTypes(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 7}}
	audio := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: "song.mp3", Type: "audio/mpeg"},
		},
	}
	AttachViewToken(d, "default", "/song.mp3", audio)
	if audio.ViewToken == "" {
		t.Fatal("expected view token on audio file")
	}
	doc := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: "readme.txt", Type: "text/plain"},
		},
	}
	AttachViewToken(d, "default", "/readme.txt", doc)
	if doc.ViewToken == "" {
		t.Fatal("expected view token on non-media file")
	}
}

func TestAttachViewTokensForDirectory(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 7}}
	file := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Type: "directory"},
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "song.mp3", Type: "audio/mpeg"}},
				{ItemInfo: iteminfo.ItemInfo{Name: "photo.jpg", Type: "image/jpeg"}},
				{ItemInfo: iteminfo.ItemInfo{Name: "nested", Type: "directory"}},
			},
		},
	}
	AttachViewTokensForDirectory(d, "Downloads", "/media/", file)
	if file.Files[0].ViewToken == "" {
		t.Fatal("expected view token on audio file")
	}
	if file.Files[1].ViewToken == "" {
		t.Fatal("expected view token on image file")
	}
	if file.Files[2].ViewToken != "" {
		t.Fatal("did not expect token on directory child folder")
	}
	if err := ValidateViewGrant(file.Files[0].ViewToken, d, "Downloads", "/media/song.mp3"); err != nil {
		t.Fatalf("validate audio grant: %v", err)
	}
	if err := ValidateViewGrant(file.Files[1].ViewToken, d, "Downloads", "/media/photo.jpg"); err != nil {
		t.Fatalf("validate image grant: %v", err)
	}
}

func TestStreamHandlerRejectsMissingToken(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream?source=default&file=/a.mp3", nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestStreamHandlerRejectsNonMedia(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 1}}
	token, err := mintViewGrant(d, "default", "/doc.pdf")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream?source=default&file=/doc.pdf&viewToken="+token, nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for non-media, got status=%d err=%v", status, err)
	}
}

func TestViewHandlerRejectsMedia(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 1}}
	token, err := mintViewGrant(d, "default", "/clip.mp4")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/view?source=default&file=/clip.mp4&viewToken="+token, nil)
	status, err := viewHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for media on view endpoint, got status=%d err=%v", status, err)
	}
}

func TestViewHandlerRejectsMissingToken(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/view?source=default&file=/a.txt", nil)
	status, err := viewHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestStreamHandlerRejectsMultiFile(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream?source=default&file=/a.mp3&file=/b.mp3&viewToken=tok", nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for multi-file, got status=%d err=%v", status, err)
	}
}

func TestStreamHandlerRejectsArchiveParams(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream?source=default&file=/a.mp3&viewToken=tok&algo=zip", nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for algo param, got status=%d err=%v", status, err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/media/stream?source=default&file=/a.mp3&viewToken=tok&archiveToken=abc", nil)
	status, err = streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for archiveToken param, got status=%d err=%v", status, err)
	}
}

func TestViewHandlerRejectsArchiveParams(t *testing.T) {
	t.Parallel()
	d := &requestContext{User: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/view?source=default&file=/a.txt&viewToken=tok&algo=zip", nil)
	status, err := viewHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for algo param, got status=%d err=%v", status, err)
	}
}
