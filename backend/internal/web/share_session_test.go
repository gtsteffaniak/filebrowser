package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestAuthenticateShareRequestUISessionCookie(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	link := share.Share{
		ShareColumns: share.ShareColumns{Hash: "ui_session_share"},
		PasswordHash: string(passwordHash),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/api/resources?hash=ui_session_share", nil)
	req.Header.Set("X-SHARE-PASSWORD", "secret")
	if err := SetShareUISessionCookie(rec, req, link.Hash); err != nil {
		t.Fatalf("SetShareUISessionCookie: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != shareUISessionCookieName {
		t.Fatalf("expected share session cookie, got %#v", cookies)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash=ui_session_share", nil)
	downloadReq.AddCookie(cookies[0])
	status, err := AuthenticateShareRequest(downloadReq, link)
	if err != nil || status != http.StatusOK {
		t.Fatalf("expected cookie auth to succeed, status=%d err=%v", status, err)
	}

	token, _, err := mintShareUISessionToken("other_share", time.Hour)
	if err != nil {
		t.Fatalf("MintShareUISessionToken: %v", err)
	}
	wrongReq := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash=ui_session_share", nil)
	wrongReq.AddCookie(&http.Cookie{Name: shareUISessionCookieName, Value: token})
	status, err = AuthenticateShareRequest(wrongReq, link)
	if err != nil || status == http.StatusOK {
		t.Fatalf("expected mismatched hash cookie to fail, status=%d err=%v", status, err)
	}
}
