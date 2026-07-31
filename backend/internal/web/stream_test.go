package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

var streamTestResolverMu sync.Mutex

func initStreamTestSources(t *testing.T) {
	t.Helper()
	streamTestResolverMu.Lock()
	t.Cleanup(func() { streamTestResolverMu.Unlock() })
	users.SetSourceNameResolver(func(name string) (string, error) {
		switch name {
		case "default", "Downloads", "srv":
			return "/" + name, nil
		default:
			return "", fmt.Errorf("unknown source %q", name)
		}
	})
	users.SetSourceConfig(&users.SourceConfigProvider{
		GetSourceByPath: func(path string) (users.SourceInfo, bool) {
			switch path {
			case "/default", "/Downloads", "/srv":
				return users.SourceInfo{Path: path, Name: path[1:]}, true
			default:
				return users.SourceInfo{}, false
			}
		},
		GetSourceByName: func(name string) (users.SourceInfo, bool) {
			switch name {
			case "default", "Downloads", "srv":
				return users.SourceInfo{Path: "/" + name, Name: name}, true
			default:
				return users.SourceInfo{}, false
			}
		},
	})
}

func testUserWithView(id uint64, sources ...string) *users.User {
	scopes := make([]users.BackendScope, 0, len(sources))
	perms := map[string]users.SourceFilePermissions{}
	for _, s := range sources {
		sourcePath := "/" + s
		sourcePerms := users.SourceFilePermissions{View: true, Download: true}
		perms[sourcePath] = sourcePerms
		scopes = append(scopes, users.BackendScope{
			Path:        sourcePath,
			Scope:       "/",
			Permissions: sourcePerms,
		})
	}
	return &users.User{
		ID: id,
		FrontendUser: users.FrontendUser{
			Username: "alice",
		},
		BackendScopes:            scopes,
		BackendSourcePermissions: perms,
		Version:                  users.SourcePermissionsMigrationVersion,
	}
}

func TestMintAndValidateViewGrant(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(42, "default")}
	token, err := mintViewGrant(d, "default")
	if err != nil {
		t.Fatalf("mintViewGrant: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if err := ValidateViewGrant(token, d, "default"); err != nil {
		t.Fatalf("ValidateViewGrant: %v", err)
	}
}

func TestValidateViewGrantWrongScope(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/default": {Path: "/default", Name: "default"},
	}
	t.Cleanup(func() { settings.Config.Server.SourceMap = nil })
	d := &requestContext{
		User: testUserWithView(1, "default"),
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "abc123"},
			SourcePath:   "/default",
		},
	}
	token, err := mintViewGrant(d, "")
	if err != nil {
		t.Fatal(err)
	}
	plain := &requestContext{User: testUserWithView(2, "default")}
	if err := ValidateViewGrant(token, plain, "default"); err == nil {
		t.Fatal("expected scope mismatch error")
	}
}

func TestValidateViewGrantAnyPathInSource(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
	token, err := mintViewGrant(d, "default")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateViewGrant(token, d, "default"); err != nil {
		t.Fatalf("source-scoped grant should validate regardless of requested file: %v", err)
	}
}

func TestMintViewGrantReusesToken(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
	first, err := mintViewGrant(d, "default")
	if err != nil {
		t.Fatal(err)
	}
	second, err := mintViewGrant(d, "default")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("expected same view token, got %q and %q", first, second)
	}
}

func TestValidateViewGrantExpired(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	token, err := utils.RandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	utils.ViewGrantsCache.Set(token, utils.ViewGrant{
		Source:    "default",
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	})
	d := &requestContext{User: testUserWithView(1, "default")}
	if err := ValidateViewGrant(token, d, "default"); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestValidateViewGrantExtendsExpiry(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	token, err := utils.RandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	almostExpired := time.Now().Add(30 * time.Second).Unix()
	utils.ViewGrantsCache.Set(token, utils.ViewGrant{
		Source:    "default",
		ExpiresAt: almostExpired,
	})
	d := &requestContext{User: testUserWithView(1, "default")}
	if err := ValidateViewGrant(token, d, "default"); err != nil {
		t.Fatalf("ValidateViewGrant: %v", err)
	}
	grant, ok := utils.ViewGrantsCache.Get(token)
	if !ok {
		t.Fatal("grant missing from cache after validate")
	}
	if grant.ExpiresAt <= almostExpired {
		t.Fatalf("ExpiresAt = %d, want extension beyond %d", grant.ExpiresAt, almostExpired)
	}
}

func TestValidateViewGrantShareBinding(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/srv": {Path: "/srv", Name: "srv"},
	}
	t.Cleanup(func() { settings.Config.Server.SourceMap = nil })
	d := &requestContext{
		User:  testUserWithView(1, "srv"),
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "abc123"},
			SourcePath:   "/srv",
		},
	}
	token, err := mintViewGrant(d, "")
	if err != nil {
		t.Fatal(err)
	}
	grant, ok := utils.ViewGrantsCache.Get(token)
	if !ok || grant.Source != "abc123" {
		t.Fatalf("expected grant scoped to share hash, got %+v ok=%v", grant, ok)
	}
	if err := ValidateViewGrant(token, d, ""); err != nil {
		t.Fatalf("ValidateViewGrant: %v", err)
	}
	wrongShare := &requestContext{
		User:  testUserWithView(1, "srv"),
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "other"},
			SourcePath:   "/srv",
		},
	}
	if err := ValidateViewGrant(token, wrongShare, ""); err == nil {
		t.Fatal("expected share mismatch error")
	}
}

func TestAttachViewTokenForAllFileTypes(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(7, "default")}
	audio := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: "song.mp3", Type: "audio/mpeg"},
		},
	}
	AttachViewToken(d, "default", audio)
	if audio.ViewToken == "" {
		t.Fatal("expected view token on audio file")
	}
	doc := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: "readme.txt", Type: "text/plain"},
		},
	}
	AttachViewToken(d, "default", doc)
	if doc.ViewToken == "" {
		t.Fatal("expected view token on non-media file")
	}
}

func TestAttachViewTokensForDirectory(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(7, "Downloads")}
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
	AttachViewTokensForDirectory(d, "Downloads", file)
	if file.Files[0].ViewToken == "" {
		t.Fatal("expected view token on audio file")
	}
	if file.Files[1].ViewToken == "" {
		t.Fatal("expected view token on image file")
	}
	if file.Files[0].ViewToken != file.Files[1].ViewToken {
		t.Fatal("expected the same universal view token for directory siblings")
	}
	if file.Files[2].ViewToken != "" {
		t.Fatal("did not expect token on directory child folder")
	}
	if err := ValidateViewGrant(file.Files[0].ViewToken, d, "Downloads"); err != nil {
		t.Fatalf("validate audio grant: %v", err)
	}
	if err := ValidateViewGrant(file.Files[1].ViewToken, d, "Downloads"); err != nil {
		t.Fatalf("validate image grant: %v", err)
	}
}

func TestAttachViewTokenDeniedWithoutViewPermission(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{
		User: &users.User{
			ID: 9,
			BackendSourcePermissions: map[string]users.SourceFilePermissions{
				"/default": {View: false, Download: true},
			},
			Version: users.SourcePermissionsMigrationVersion,
		},
	}
	doc := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: "readme.txt", Type: "text/plain"},
		},
	}
	AttachViewToken(d, "default", doc)
	if doc.ViewToken != "" {
		t.Fatal("expected no view token without view permission")
	}
}

func TestStreamHandlerRejectsMissingToken(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream?source=default&file=/a.mp3", nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestStreamHandlerRejectsNonMedia(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
	token, err := mintViewGrant(d, "default")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream?source=default&file=/doc.pdf&viewToken="+token, nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for non-media, got status=%d err=%v", status, err)
	}
}

func TestShareRelativeDisplayName(t *testing.T) {
	t.Parallel()
	d := &requestContext{
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "abc", Path: "/users/alice/media/song.mp3"},
		},
	}
	if got := shareRelativeDisplayName(d, "/"); got != "song.mp3" {
		t.Fatalf("shareRelativeDisplayName(/) = %q, want song.mp3", got)
	}
	if got := shareRelativeDisplayName(d, "/nested/other.mp3"); got != "other.mp3" {
		t.Fatalf("shareRelativeDisplayName(/nested/other.mp3) = %q, want other.mp3", got)
	}
}

func TestPublicStreamHandlerSingleFileShareRootPath(t *testing.T) {
	// SourceMap is process-global; save/restore and avoid t.Parallel (see streamTestResolverMu).
	initStreamTestSources(t)
	prevSourceMap := settings.Config.Server.SourceMap
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/srv": {Path: "/srv", Name: "srv"},
	}
	t.Cleanup(func() { settings.Config.Server.SourceMap = prevSourceMap })
	d := &requestContext{
		User: testUserWithView(1, "srv"),
		Share: share.Share{
			ShareColumns: share.ShareColumns{
				Hash: "abc123",
				Path: "/scope/media/song.mp3",
			},
			SourcePath: "/srv",
			ShareSettings: share.ShareSettings{
				FrontendShareInfo: share.FrontendShareInfo{
					DisableFileViewer: false,
				},
			},
		},
	}
	token, err := mintViewGrant(d, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/public/api/media/stream?hash=abc123&file=/&viewToken="+token, nil)
	_, err = publicStreamHandler(httptest.NewRecorder(), req, d)
	if err != nil && strings.Contains(err.Error(), "stream endpoint supports audio and video only") {
		t.Fatalf("single-file share root path rejected as non-media: %v", err)
	}
}

func TestViewHandlerRejectsMedia(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
	token, err := mintViewGrant(d, "default")
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
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/view?source=default&file=/a.txt", nil)
	status, err := viewHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}
}

func TestStreamHandlerRejectsMultiFile(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream?source=default&file=/a.mp3&file=/b.mp3&viewToken=tok", nil)
	status, err := streamHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for multi-file, got status=%d err=%v", status, err)
	}
}

func TestStreamHandlerRejectsArchiveParams(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
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
	initStreamTestSources(t)
	d := &requestContext{User: testUserWithView(1, "default")}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/view?source=default&file=/a.txt&viewToken=tok&algo=zip", nil)
	status, err := viewHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for algo param, got status=%d err=%v", status, err)
	}
}

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
	mediaCtx := &requestContext{User: &users.User{FrontendUser: users.FrontendUser{Permissions: users.Permissions{Download: true}}}}
	if !streamUseRangeOnly(mediaCtx, "clip.mp4") {
		t.Fatal("expected range-only for stream endpoint")
	}
	if !streamUseRangeOnly(mediaCtx, "notes.txt") {
		t.Fatal("stream endpoint is always range-only")
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
	if !IsMediaStreamFile("movie.mp4") || !IsMediaStreamFile("track.flac") {
		t.Fatal("expected media extensions to match")
	}
	if IsMediaStreamFile("readme.txt") {
		t.Fatal("did not expect text file to match")
	}
}

func TestViewTokenHandlerMintsUniversalGrant(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{
		User:  testUserWithView(1, "default"),
		Token: "session-jwt-not-in-tokens",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/resources/view-token?source=default", nil)
	rec := httptest.NewRecorder()
	status, err := viewTokenHandler(rec, req, d)
	if err != nil || status != http.StatusOK {
		t.Fatalf("viewTokenHandler: status=%d err=%v", status, err)
	}
	var resp viewTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ViewToken == "" || resp.ExpiresAt == 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if err := ValidateViewGrant(resp.ViewToken, d, "default"); err != nil {
		t.Fatalf("ValidateViewGrant: %v", err)
	}
}

func TestViewTokenHandlerMintsOnShareWithoutWebSession(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/srv": {Path: "/srv", Name: "srv"},
	}
	t.Cleanup(func() { settings.Config.Server.SourceMap = nil })
	d := &requestContext{
		User: &users.User{
			FrontendUser: users.FrontendUser{Username: "anonymous"},
		},
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "abc123"},
			SourcePath:   "/srv",
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/public/api/resources/view-token?hash=abc123", nil)
	rec := httptest.NewRecorder()
	status, err := viewTokenHandler(rec, req, d)
	if err != nil || status != http.StatusOK {
		t.Fatalf("viewTokenHandler: status=%d err=%v", status, err)
	}
	var resp viewTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ViewToken == "" {
		t.Fatal("expected view token on share route")
	}
	if err := ValidateViewGrant(resp.ViewToken, d, ""); err != nil {
		t.Fatalf("ValidateViewGrant: %v", err)
	}
	grant, ok := utils.ViewGrantsCache.Get(resp.ViewToken)
	if !ok || grant.Source != "abc123" {
		t.Fatalf("expected share hash scope, got %+v ok=%v", grant, ok)
	}
}

func TestViewTokenHandlerRejectsSourceOnShare(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/srv": {Path: "/srv", Name: "srv"},
	}
	t.Cleanup(func() { settings.Config.Server.SourceMap = nil })
	d := &requestContext{
		User: &users.User{
			FrontendUser: users.FrontendUser{Username: "anonymous"},
		},
		Share: share.Share{
			ShareColumns: share.ShareColumns{Hash: "abc123"},
			SourcePath:   "/srv",
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/public/api/resources/view-token?hash=abc123&source=srv", nil)
	status, err := viewTokenHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusBadRequest || err == nil {
		t.Fatalf("expected 400 when source supplied on share, got status=%d err=%v", status, err)
	}
}

func TestViewTokenHandlerRejectsNamedApiToken(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	user := testUserWithView(1, "default")
	user.Tokens = map[string]users.AuthToken{}
	users.StoreToken(user.Tokens, users.AuthToken{Name: "my-api-key", Token: "api-jwt-value"})
	d := &requestContext{
		User:  user,
		Token: "api-jwt-value",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/resources/view-token?source=default", nil)
	status, err := viewTokenHandler(httptest.NewRecorder(), req, d)
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected 403 for API token, got status=%d err=%v", status, err)
	}
}

func TestViewTokenHandlerExtendsExistingToken(t *testing.T) {
	t.Parallel()
	initStreamTestSources(t)
	d := &requestContext{
		User:  testUserWithView(1, "default"),
		Token: "session-jwt",
	}
	token, err := utils.RandomHex(16)
	if err != nil {
		t.Fatal(err)
	}
	almostExpired := time.Now().Add(30 * time.Second).Unix()
	utils.ViewGrantsCache.Set(token, utils.ViewGrant{
		Source:    viewGrantScope(d, "default"),
		ExpiresAt: almostExpired,
	})
	utils.ViewGrantIndex.Set(viewGrantScope(d, "default"), token)
	req := httptest.NewRequest(http.MethodPost, "/api/resources/view-token?source=default&viewToken="+token, nil)
	rec := httptest.NewRecorder()
	status, err := viewTokenHandler(rec, req, d)
	if err != nil || status != http.StatusOK {
		t.Fatalf("viewTokenHandler: status=%d err=%v", status, err)
	}
	var resp viewTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ViewToken != token {
		t.Fatalf("expected same token extended, got %q", resp.ViewToken)
	}
	grantAfter, _ := utils.ViewGrantsCache.Get(token)
	if grantAfter.ExpiresAt <= almostExpired {
		t.Fatalf("expected extended expiry, before=%d after=%d", almostExpired, grantAfter.ExpiresAt)
	}
}
