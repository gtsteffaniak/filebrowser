package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestSessionCookie_HttpOnlyAndSecureOnHTTPS(t *testing.T) {
	prev := settings.Config.Http.TrustProxyHeaders
	settings.Config.Http.TrustProxyHeaders = true
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = prev })

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
	if csp != "script-src 'self' 'nonce-testnonce' https://cdn.jsdelivr.net https://www.google.com https://www.gstatic.com" {
		t.Fatalf("unexpected CSP: %s", csp)
	}
	if strings.Contains(csp, "'unsafe-inline'") {
		t.Fatal("script-src must not allow unsafe-inline (srcdoc XSS)")
	}
	if strings.Contains(csp, "default-src") {
		t.Fatal("CSP should only restrict script-src to avoid breaking frames, blobs, and external assets")
	}
}
