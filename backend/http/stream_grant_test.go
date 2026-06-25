package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func TestMintAndValidateStreamGrant(t *testing.T) {
	t.Parallel()
	d := &requestContext{
		user: &users.User{ID: 42, FrontendUser: users.FrontendUser{Username: "alice"}},
	}
	token, err := mintStreamGrant(d, "default", "/docs/readme.txt")
	if err != nil {
		t.Fatalf("mintStreamGrant: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if err := validateStreamGrant(token, d, "default", "/docs/readme.txt"); err != nil {
		t.Fatalf("validateStreamGrant: %v", err)
	}
}

func TestValidateStreamGrantWrongUser(t *testing.T) {
	t.Parallel()
	owner := &requestContext{user: &users.User{ID: 1}}
	other := &requestContext{user: &users.User{ID: 2}}
	token, err := mintStreamGrant(owner, "default", "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateStreamGrant(token, other, "default", "/a.txt"); err == nil {
		t.Fatal("expected viewer mismatch error")
	}
}

func TestValidateStreamGrantWrongPath(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: &users.User{ID: 1}}
	token, err := mintStreamGrant(d, "default", "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateStreamGrant(token, d, "default", "/b.txt"); err == nil {
		t.Fatal("expected path mismatch error")
	}
}

func TestValidateStreamGrantExpired(t *testing.T) {
	t.Parallel()
	token, err := utils.RandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	utils.StreamGrantsCache.Set(token, utils.StreamGrant{
		UserID:    1,
		Source:    "default",
		Path:      "/a.txt",
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	})
	d := &requestContext{user: &users.User{ID: 1}}
	if err := validateStreamGrant(token, d, "default", "/a.txt"); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestValidateStreamGrantShareBinding(t *testing.T) {
	t.Parallel()
	d := &requestContext{
		user:  &users.User{ID: 1},
		share: share.Share{ShareColumns: share.ShareColumns{Hash: "abc123"}},
	}
	token, err := mintStreamGrant(d, "srv", "/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	wrongShare := &requestContext{
		user:  &users.User{ID: 1},
		share: share.Share{ShareColumns: share.ShareColumns{Hash: "other"}},
	}
	if err := validateStreamGrant(token, wrongShare, "srv", "/file.txt"); err == nil {
		t.Fatal("expected share mismatch error")
	}
}


func TestAttachStreamTokensForDirectory(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: &users.User{ID: 7}}
	file := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Type: "directory"},
			Files: []iteminfo.ExtendedItemInfo{
				{ItemInfo: iteminfo.ItemInfo{Name: "clip.mp4", Type: "video/mp4"}},
				{ItemInfo: iteminfo.ItemInfo{Name: "nested", Type: "directory"}},
			},
		},
	}
	attachStreamTokensForDirectory(d, "Downloads", "/photos/", file)
	if file.Files[0].StreamToken == "" {
		t.Fatal("expected stream token on directory child file")
	}
	if file.Files[1].StreamToken != "" {
		t.Fatal("did not expect stream token on directory child folder")
	}
	token := file.Files[0].StreamToken
	if err := validateStreamGrant(token, d, "Downloads", "/photos/clip.mp4"); err != nil {
		t.Fatalf("validate child grant: %v", err)
	}
}

func TestStreamHandlerRejectsMissingToken(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/stream?source=default&file=/a.txt", nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestStreamHandlerRejectsMultiFile(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/stream?source=default&file=/a.txt&file=/b.txt&streamToken=tok", nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for multi-file, got status=%d err=%v", status, err)
	}
}

func TestStreamHandlerRejectsArchiveParams(t *testing.T) {
	t.Parallel()
	d := &requestContext{user: &users.User{ID: 1}}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/stream?source=default&file=/a.txt&streamToken=tok&algo=zip", nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for algo param, got status=%d err=%v", status, err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/resources/stream?source=default&file=/a.txt&streamToken=tok&archiveToken=abc", nil)
	status, err = streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for archiveToken param, got status=%d err=%v", status, err)
	}
}
