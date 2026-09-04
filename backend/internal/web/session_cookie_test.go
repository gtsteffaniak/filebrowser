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
	prev := settings.Config.Integrations.OnlyOffice
	settings.Config.Integrations.OnlyOffice = settings.OnlyOffice{}
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = prev })

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

func TestSpaContentSecurityPolicy_IncludesOnlyOfficeOrigins(t *testing.T) {
	prev := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = prev })

	tests := []struct {
		name string
		oo   settings.OnlyOffice
		want string
	}{
		{
			name: "url only",
			oo:   settings.OnlyOffice{Url: "http://localhost:9052"},
			want: "script-src 'self' 'nonce-testnonce' https://cdn.jsdelivr.net https://www.google.com https://www.gstatic.com http://localhost:9052",
		},
		{
			name: "internal url only",
			oo:   settings.OnlyOffice{InternalUrl: "http://onlyoffice-internal:80"},
			want: "script-src 'self' 'nonce-testnonce' https://cdn.jsdelivr.net https://www.google.com https://www.gstatic.com http://onlyoffice-internal:80",
		},
		{
			name: "both urls",
			oo: settings.OnlyOffice{
				Url:         "https://office.example.com",
				InternalUrl: "http://onlyoffice-internal",
			},
			want: "script-src 'self' 'nonce-testnonce' https://cdn.jsdelivr.net https://www.google.com https://www.gstatic.com https://office.example.com http://onlyoffice-internal",
		},
		{
			name: "duplicate origins",
			oo: settings.OnlyOffice{
				Url:         "http://localhost:9052",
				InternalUrl: "http://localhost:9052",
			},
			want: "script-src 'self' 'nonce-testnonce' https://cdn.jsdelivr.net https://www.google.com https://www.gstatic.com http://localhost:9052",
		},
		{
			name: "invalid urls ignored",
			oo: settings.OnlyOffice{
				Url:         "not-a-url",
				InternalUrl: "://bad",
			},
			want: "script-src 'self' 'nonce-testnonce' https://cdn.jsdelivr.net https://www.google.com https://www.gstatic.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.Config.Integrations.OnlyOffice = tt.oo
			if got := spaContentSecurityPolicy("testnonce"); got != tt.want {
				t.Fatalf("unexpected CSP:\ngot:  %s\nwant: %s", got, tt.want)
			}
		})
	}
}
