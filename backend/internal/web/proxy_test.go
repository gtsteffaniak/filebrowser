package web

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestGetScheme(t *testing.T) {
	t.Run("trusted proto", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = true
		t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = false })

		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		if got := GetScheme(req); got != "https" {
			t.Fatalf("GetScheme = %q, want https", got)
		}
	})

	t.Run("ignores spoofed proto when untrusted", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = false

		req := httptest.NewRequest("GET", "http://example.com/", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		if got := GetScheme(req); got != "http" {
			t.Fatalf("GetScheme = %q, want http", got)
		}
	})
}

func TestRequestSchemeTLSFallback(t *testing.T) {
	settings.Config.Http.TrustProxyHeaders = false

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.TLS = &tls.ConnectionState{}
	if got := requestScheme(req); got != "https" {
		t.Fatalf("requestScheme = %q, want https", got)
	}
}

func TestRequestSchemeIgnoresInvalidProto(t *testing.T) {
	settings.Config.Http.TrustProxyHeaders = true
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = false })

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Forwarded-Proto", "javascript")
	if got := requestScheme(req); got != "http" {
		t.Fatalf("requestScheme = %q, want http", got)
	}
}

func TestRequestHostTrustProxyHeaders(t *testing.T) {
	settings.Config.Http.TrustProxyHeaders = true
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = false })

	req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-Host", "public.example.com, internal.example.com")

	if got := requestHost(req); got != "public.example.com" {
		t.Fatalf("requestHost = %q, want public.example.com", got)
	}
}

func TestRequestSchemeForPublicURL(t *testing.T) {
	orig := settings.Config.Http.TrustProxyHeaders
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = orig })

	t.Run("trusted host without proto defaults to https", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = true
		req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
		req.Header.Set("X-Forwarded-Host", "public.example.com")
		if got := requestSchemeForPublicURL(req); got != "https" {
			t.Fatalf("requestSchemeForPublicURL = %q, want https", got)
		}
	})

	t.Run("trusted host with proto honors proto", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = true
		req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
		req.Header.Set("X-Forwarded-Host", "public.example.com")
		req.Header.Set("X-Forwarded-Proto", "http")
		if got := requestSchemeForPublicURL(req); got != "http" {
			t.Fatalf("requestSchemeForPublicURL = %q, want http", got)
		}
	})

	t.Run("ignores forwarded proto when untrusted", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = false
		req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
		req.Header.Set("X-Forwarded-Proto", "https")
		if got := requestSchemeForPublicURL(req); got != "http" {
			t.Fatalf("requestSchemeForPublicURL = %q, want http", got)
		}
	})
}

func TestRequestPageFullURL(t *testing.T) {
	orig := settings.Config.Http
	t.Cleanup(func() { settings.Config.Http = orig })
	settings.Config.Http.ExternalUrl = ""

	tests := []struct {
		name           string
		directHost     string
		forwardedHost  string
		forwardedProto string
		trustProxy     bool
		path           string
		want           string
	}{
		{
			name:           "trusted forwarded host and proto",
			directHost:     "127.0.0.1:8080",
			forwardedHost:  "files.example.com",
			forwardedProto: "https",
			trustProxy:     true,
			path:           "/files/",
			want:           "https://files.example.com/files/",
		},
		{
			name:          "trusted host without proto defaults to https",
			directHost:    "127.0.0.1:8080",
			forwardedHost: "files.example.com",
			trustProxy:    true,
			path:          "/files/",
			want:          "https://files.example.com/files/",
		},
		{
			name:           "trusted host with proto honors proto",
			directHost:     "127.0.0.1:8080",
			forwardedHost:  "files.example.com",
			forwardedProto: "http",
			trustProxy:     true,
			path:           "/files/",
			want:           "http://files.example.com/files/",
		},
		{
			name:           "ignores spoofed host when untrusted",
			directHost:     "127.0.0.1:8080",
			forwardedHost:  "evil.example.com",
			forwardedProto: "https",
			trustProxy:     false,
			path:           "/files/",
			want:           "http://127.0.0.1:8080/files/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings.Config.Http.TrustProxyHeaders = tt.trustProxy
			req := httptest.NewRequest("GET", "http://example.com"+tt.path, nil)
			req.Host = tt.directHost
			if tt.forwardedHost != "" {
				req.Header.Set("X-Forwarded-Host", tt.forwardedHost)
			}
			if tt.forwardedProto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if got := requestPageFullURL(req); got != tt.want {
				t.Fatalf("requestPageFullURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
