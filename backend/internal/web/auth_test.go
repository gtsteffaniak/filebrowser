package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func findSessionCookie(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == "filebrowser_quantum_jwt" {
			return c
		}
	}
	t.Fatal("session cookie not found")
	return nil
}

func TestSetSessionCookieUsesTrustedHost(t *testing.T) {
	orig := settings.Config.Http.TrustProxyHeaders
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = orig })

	settings.Config.Http.TrustProxyHeaders = true
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-Host", "files.example.com")

	rec := httptest.NewRecorder()
	SetSessionCookie(rec, req, "token", time.Now().Add(time.Hour))

	cookie := findSessionCookie(t, rec)
	if cookie.Domain != "files.example.com" {
		t.Fatalf("cookie Domain = %q, want files.example.com", cookie.Domain)
	}
}

func TestSetSessionCookieIgnoresUntrustedForwardedHost(t *testing.T) {
	settings.Config.Http.TrustProxyHeaders = false
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = false })

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-Host", "evil.example.com")

	rec := httptest.NewRecorder()
	SetSessionCookie(rec, req, "token", time.Now().Add(time.Hour))

	cookie := findSessionCookie(t, rec)
	if cookie.Domain != "127.0.0.1" {
		t.Fatalf("cookie Domain = %q, want 127.0.0.1", cookie.Domain)
	}
}

func TestLogoutClearsCookieForTrustedHost(t *testing.T) {
	setupTestEnv(t)

	origTrust := settings.Config.Http.TrustProxyHeaders
	origBaseURL := settings.Config.Http.BaseURL
	t.Cleanup(func() {
		settings.Config.Http.TrustProxyHeaders = origTrust
		settings.Config.Http.BaseURL = origBaseURL
	})

	settings.Config.Http.TrustProxyHeaders = true
	settings.Config.Http.BaseURL = "/"

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-Host", "files.example.com")

	rec := httptest.NewRecorder()
	status, err := logoutHandler(rec, req, &Context{User: &users.User{
		FrontendUser: users.FrontendUser{LoginMethod: users.LoginMethodPassword},
	}})
	if err != nil || status != http.StatusOK {
		t.Fatalf("logoutHandler() status=%d err=%v", status, err)
	}

	cookie := findSessionCookie(t, rec)
	if cookie.Domain != "files.example.com" {
		t.Fatalf("logout cookie Domain = %q, want files.example.com", cookie.Domain)
	}
	if cookie.MaxAge != -1 {
		t.Fatalf("logout cookie MaxAge = %d, want -1", cookie.MaxAge)
	}
}

func TestExtractTokenPrefersAuthorizationOverCookie(t *testing.T) {
	const apiToken = "aaa.bbb.ccc"
	const cookieToken = "revoked.header.token"

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{
		Name:  "filebrowser_quantum_jwt",
		Value: cookieToken,
	})
	req.Header.Set("Authorization", "Bearer "+apiToken)

	got, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("ExtractToken() error = %v", err)
	}
	if got != apiToken {
		t.Fatalf("ExtractToken() = %q, want %q", got, apiToken)
	}
}

func TestExtractTokenUsesCookieWhenNoAuthorizationHeader(t *testing.T) {
	const cookieToken = "session.header.token"

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{
		Name:  "filebrowser_quantum_jwt",
		Value: cookieToken,
	})

	got, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("ExtractToken() error = %v", err)
	}
	if got != cookieToken {
		t.Fatalf("ExtractToken() = %q, want %q", got, cookieToken)
	}
}

func TestExtractTokenUsesAuthQueryBeforeCookie(t *testing.T) {
	const queryToken = "query.header.token"
	const cookieToken = "session.header.token"

	req := httptest.NewRequest(http.MethodGet, "/api/users?auth="+queryToken, nil)
	req.AddCookie(&http.Cookie{
		Name:  "filebrowser_quantum_jwt",
		Value: cookieToken,
	})

	got, err := ExtractToken(req)
	if err != nil {
		t.Fatalf("ExtractToken() error = %v", err)
	}
	if got != queryToken {
		t.Fatalf("ExtractToken() = %q, want %q", got, queryToken)
	}
}

func TestApplyNamedApiTokenGlobalCaps(t *testing.T) {
	owner := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "alice",
			Permissions: users.Permissions{
				Admin: true,
				Api:   true,
				Share: true,
			},
		},
		ID: 1,
	}

	t.Run("session token unchanged", func(t *testing.T) {
		user := *owner
		applyNamedApiTokenGlobalCaps(&user, users.AuthToken{
			BelongsTo:   1,
			Permissions: users.Permissions{Admin: false},
		}, "WEB_TOKEN_abcd")
		if !user.Permissions.Admin {
			t.Fatal("session token should keep DB admin")
		}
	})

	t.Run("minimal api token unchanged", func(t *testing.T) {
		user := *owner
		applyNamedApiTokenGlobalCaps(&user, users.AuthToken{}, "my-key")
		if !user.Permissions.Admin {
			t.Fatal("minimal token should keep DB admin")
		}
	})

	t.Run("custom api token caps globals", func(t *testing.T) {
		user := *owner
		applyNamedApiTokenGlobalCaps(&user, users.AuthToken{
			BelongsTo: 1,
			Permissions: users.Permissions{
				Admin:  false,
				Api:    true,
				Share:  true,
				Modify: true,
			},
		}, "customized")
		if user.Permissions.Admin {
			t.Fatal("custom token should cap admin from JWT")
		}
		if !user.Permissions.Api || !user.Permissions.Share {
			t.Fatalf("expected api and share, got %#v", user.Permissions)
		}
	})
}
