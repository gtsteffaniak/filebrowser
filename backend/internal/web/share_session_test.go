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
	status, err := AuthenticateShareRequest(rec, req, link)
	if err != nil || status != http.StatusOK {
		t.Fatalf("expected password auth to succeed, status=%d err=%v", status, err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != shareUISessionCookieName {
		t.Fatalf("expected share session cookie, got %#v", cookies)
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash=ui_session_share", nil)
	downloadReq.AddCookie(cookies[0])
	status, err = AuthenticateShareRequest(nil, downloadReq, link)
	if err != nil || status != http.StatusOK {
		t.Fatalf("expected cookie auth to succeed, status=%d err=%v", status, err)
	}

	token, _, mintErr := mintShareUISessionToken("other_share", time.Hour)
	if mintErr != nil {
		t.Fatalf("mintShareUISessionToken: %v", mintErr)
	}
	wrongReq := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash=ui_session_share", nil)
	wrongReq.AddCookie(&http.Cookie{Name: shareUISessionCookieName, Value: token})
	status, err = AuthenticateShareRequest(nil, wrongReq, link)
	if err != nil || status == http.StatusOK {
		t.Fatalf("expected mismatched hash cookie to fail, status=%d err=%v", status, err)
	}
}

func TestAuthenticateShareRequestDownloadTokenDoesNotMintUISession(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	link := share.Share{
		ShareColumns: share.ShareColumns{Hash: "dl_token_share"},
		PasswordHash: string(passwordHash),
	}

	downloadToken, _, err := mintShareDownloadAccessToken(link.Hash, time.Hour, 0)
	if err != nil {
		t.Fatalf("mintShareDownloadAccessToken: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash="+link.Hash+"&token="+downloadToken, nil)
	req.Header.Set("X-SHARE-PASSWORD", "secret")
	status, err := AuthenticateShareRequest(rec, req, link)
	if err != nil || status != http.StatusOK {
		t.Fatalf("expected download token auth to succeed, status=%d err=%v", status, err)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("download token auth must not mint a UI session cookie when password header is also present")
	}
}

func TestAuthenticateShareRequestDownloadTokenWrongPasswordNoUISession(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-auth-key"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	passwordHash, bcryptErr := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)
	if bcryptErr != nil {
		t.Fatalf("bcrypt: %v", bcryptErr)
	}
	link := share.Share{
		ShareColumns: share.ShareColumns{Hash: "dl_wrong_pw_share"},
		PasswordHash: string(passwordHash),
	}

	downloadToken, _, mintErr := mintShareDownloadAccessToken(link.Hash, time.Hour, 0)
	if mintErr != nil {
		t.Fatalf("mintShareDownloadAccessToken: %v", mintErr)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash="+link.Hash+"&token="+downloadToken, nil)
	req.Header.Set("X-SHARE-PASSWORD", "wrong-password")
	status, authErr := AuthenticateShareRequest(rec, req, link)
	if authErr != nil || status != http.StatusOK {
		t.Fatalf("expected download token auth to succeed, status=%d err=%v", status, authErr)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("valid download token with wrong password header must not mint a UI session cookie")
	}
}
