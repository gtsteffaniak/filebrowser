package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestValidateOidcCallbackState(t *testing.T) {
	settings.Config.Http.BaseURL = "/"
	t.Cleanup(func() { settings.Config.Http.BaseURL = "/" })

	t.Run("accepts matching state and redirect cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/auth/oidc/callback?state=goodstate&code=x", nil)
		req.AddCookie(&http.Cookie{Name: oidcStateCookieName, Value: "goodstate"})
		req.AddCookie(&http.Cookie{Name: oidcRedirectCookieName, Value: "/files/"})
		rec := httptest.NewRecorder()

		redirect, status, err := validateOidcCallbackState(req, rec)
		if err != nil || status != 0 {
			t.Fatalf("validateOidcCallbackState: status=%d err=%v", status, err)
		}
		if redirect != "/files/" {
			t.Fatalf("redirect = %q, want /files/", redirect)
		}
		setCookie := rec.Header().Get("Set-Cookie")
		if !strings.Contains(setCookie, oidcStateCookieName+"=;") {
			t.Fatalf("expected state cookie cleared, got %q", setCookie)
		}
	})

	t.Run("rejects tampered state", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/auth/oidc/callback?state=tampered&code=x", nil)
		req.AddCookie(&http.Cookie{Name: oidcStateCookieName, Value: "expected"})
		rec := httptest.NewRecorder()

		_, status, err := validateOidcCallbackState(req, rec)
		if err == nil || status != http.StatusBadRequest {
			t.Fatalf("expected 400, got status=%d err=%v", status, err)
		}
	})
}

func TestParseSignupCredentialsJSON(t *testing.T) {
	body := `{"username":"alice","password":"s3cret"}`
	req := httptest.NewRequest("POST", "/api/auth/signup", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	user, pass, err := parseSignupCredentials(req)
	if err != nil {
		t.Fatal(err)
	}
	if user != "alice" || pass != "s3cret" {
		t.Fatalf("got user=%q pass=%q", user, pass)
	}
}

func TestParseSignupCredentialsQueryFallback(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/auth/signup?username=bob&password=oldway", nil)
	user, pass, err := parseSignupCredentials(req)
	if err != nil {
		t.Fatal(err)
	}
	if user != "bob" || pass != "oldway" {
		t.Fatalf("got user=%q pass=%q", user, pass)
	}
}
