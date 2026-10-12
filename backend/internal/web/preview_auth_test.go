package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupPreviewGrantTest(t *testing.T) (string, string, string) {
	t.Helper()
	root, source, owner := setupShareArchiveDownloadTest(t)
	originalKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "preview-test-signing-key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalKey })
	owner.FrontendScopes = []users.FrontendScope{{
		Name: source, Scope: "/", Permissions: &users.SourceFilePermissions{View: true, Download: true},
	}}
	if err := state.UpdateUser(owner, "", "scopes"); err != nil {
		t.Fatal(err)
	}
	stored, err := state.GetUserByUsername(owner.Username)
	if err != nil {
		t.Fatal(err)
	}
	session := testSessionToken(t, &stored, time.Hour)
	grant, err := mintViewGrant(&Context{User: &stored, Token: session}, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"shared", "denied"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "movie.mp4"), []byte("0123456789"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "post.html"), []byte("<h1>page</h1>"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return source, session, grant
}

func previewRequest(t *testing.T, source, token, file string, handler handleFunc) *httptest.ResponseRecorder {
	t.Helper()
	params := url.Values{"source": {source}, "file": {file}, "viewToken": {token}}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/view?"+params.Encode(), nil)
	req.Header.Set("Range", "bytes=2-5")
	rec := httptest.NewRecorder()
	wrapHandler(withPreviewUserHelper(handler))(rec, req)
	return rec
}

func TestPreviewGrantCookieLessVideoAndHTML(t *testing.T) {
	source, session, token := setupPreviewGrantTest(t)
	video := previewRequest(t, source, token, "/shared/movie.mp4", streamHandler)
	if video.Code != http.StatusPartialContent || video.Body.String() != "2345" {
		t.Fatalf("video: status=%d body=%s", video.Code, video.Body.String())
	}
	if video.Header().Get("Set-Cookie") != "" || strings.Contains(video.Body.String(), session) {
		t.Fatal("preview response must not expose the login session")
	}
	page := previewRequest(t, source, token, "/shared/post.html", viewHandler)
	if page.Code != http.StatusPartialContent {
		t.Fatalf("HTML: %d %s", page.Code, page.Body.String())
	}
}

func TestPreviewGrantDoesNotBypassPathRules(t *testing.T) {
	source, _, token := setupPreviewGrantTest(t)
	for _, tc := range []struct {
		file    string
		handler handleFunc
	}{
		{"/denied/movie.mp4", streamHandler}, {"/denied/post.html", viewHandler},
	} {
		rec := previewRequest(t, source, token, tc.file, tc.handler)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("denied path %s: %d", tc.file, rec.Code)
		}
	}
}

func TestPreviewGrantRejectsExpiredRevokedAndWrongSource(t *testing.T) {
	source, session, token := setupPreviewGrantTest(t)
	if rec := previewRequest(t, "different", token, "/shared/movie.mp4", streamHandler); rec.Code != http.StatusForbidden {
		t.Fatalf("wrong source: %d", rec.Code)
	}
	grant, _ := utils.ViewGrantsCache.Get(token)
	expired := grant
	expired.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	utils.ViewGrantsCache.Set("expired-preview-test", expired)
	if rec := previewRequest(t, source, "expired-preview-test", "/shared/movie.mp4", streamHandler); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired grant: %d", rec.Code)
	}
	if err := state.RevokeToken(session); err != nil {
		t.Fatal(err)
	}
	if rec := previewRequest(t, source, token, "/shared/movie.mp4", streamHandler); rec.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session: %d", rec.Code)
	}
}

func TestPreviewGrantIsSessionBound(t *testing.T) {
	source, session, first := setupPreviewGrantTest(t)
	owner, err := state.GetUserByUsername("owner")
	if err != nil {
		t.Fatal(err)
	}
	secondSession := testSessionToken(t, &owner, 2*time.Hour)
	if session == secondSession {
		t.Fatal("expected distinct sessions")
	}
	second, err := mintViewGrant(&Context{User: &owner, Token: secondSession}, source)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("sessions must not share private viewing grants")
	}
	if err := ValidateViewGrant(first, &Context{User: &owner, Token: secondSession}, source); err == nil {
		t.Fatal("a grant from another session must not validate")
	}
}

func TestPreviewGrantDoesNotAuthenticateOtherRoutesOrInvalidCredentials(t *testing.T) {
	source, _, token := setupPreviewGrantTest(t)
	params := url.Values{"source": {source}, "file": {"/shared/movie.mp4"}, "viewToken": {token}}
	req := httptest.NewRequest(http.MethodGet, "/api/users?"+params.Encode(), nil)
	rec := httptest.NewRecorder()
	withUser(mockHandler)(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ordinary route: %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/media/stream?"+params.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "invalid"})
	rec = httptest.NewRecorder()
	wrapHandler(withPreviewUserHelper(streamHandler))(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid explicit credentials: %d", rec.Code)
	}
}

func TestPreviewGrantRechecksCurrentUserPermissions(t *testing.T) {
	source, _, token := setupPreviewGrantTest(t)
	owner, err := state.GetUserByUsername("owner")
	if err != nil {
		t.Fatal(err)
	}
	owner.FrontendScopes = []users.FrontendScope{{
		Name: source, Scope: "/", Permissions: &users.SourceFilePermissions{},
	}}
	if err := state.UpdateUser(&owner, "", "scopes"); err != nil {
		t.Fatal(err)
	}
	if rec := previewRequest(t, source, token, "/shared/movie.mp4", streamHandler); rec.Code != http.StatusForbidden {
		t.Fatalf("revoked view permission: %d", rec.Code)
	}
}
