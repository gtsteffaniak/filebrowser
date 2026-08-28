package http

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSessionCookie_HttpOnlyAndSecureOnHTTPS(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Forwarded-Proto", "https")
	req.Host = "example.com"

	c := sessionCookie(req, "token", time.Now().Add(time.Hour), 0, http.SameSiteStrictMode)
	if !c.HttpOnly {
		t.Fatal("expected HttpOnly session cookie")
	}
	if !c.Secure {
		t.Fatal("expected Secure session cookie over HTTPS")
	}
	if c.Name != sessionCookieName {
		t.Fatalf("cookie name: got %q", c.Name)
	}
}

func TestSessionCookie_NotSecureOnHTTP(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://localhost/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = "localhost"

	c := sessionCookie(req, "token", time.Now().Add(time.Hour), 0, http.SameSiteStrictMode)
	if !c.HttpOnly {
		t.Fatal("expected HttpOnly session cookie")
	}
	if c.Secure {
		t.Fatal("did not expect Secure cookie on plain HTTP")
	}
}

func TestSpaContentSecurityPolicy_BlocksUntrustedScripts(t *testing.T) {
	csp := spaContentSecurityPolicy("testnonce")
	if !strings.Contains(csp, "script-src 'self' 'nonce-testnonce'") {
		t.Fatalf("missing nonce'd script-src: %s", csp)
	}
	for _, dir := range strings.Split(csp, ";") {
		d := strings.TrimSpace(dir)
		if strings.HasPrefix(d, "script-src ") && strings.Contains(d, "'unsafe-inline'") {
			t.Fatal("script-src must not allow unsafe-inline (srcdoc XSS)")
		}
	}
}
