package web

import (
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestDeriveRPIDTrustProxyHeaders(t *testing.T) {
	orig := settings.Config.Http.TrustProxyHeaders
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = orig })

	t.Run("trusted forwarded host", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = true
		req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
		req.Host = "127.0.0.1:8080"
		req.Header.Set("X-Forwarded-Host", "files.example.com")

		if got := deriveRPID(req); got != "files.example.com" {
			t.Fatalf("deriveRPID() = %q, want files.example.com", got)
		}
	})

	t.Run("ignores untrusted forwarded host", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = false
		req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
		req.Host = "127.0.0.1:8080"
		req.Header.Set("X-Forwarded-Host", "evil.example.com")

		if got := deriveRPID(req); got != "127.0.0.1" {
			t.Fatalf("deriveRPID() = %q, want 127.0.0.1", got)
		}
	})
}

func TestWebAuthnRequestOriginTrustProxyHeaders(t *testing.T) {
	orig := settings.Config.Http.TrustProxyHeaders
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = orig })

	t.Run("trusted forwarded host and proto", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = true
		req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
		req.Host = "127.0.0.1:8080"
		req.Header.Set("X-Forwarded-Host", "files.example.com")
		req.Header.Set("X-Forwarded-Proto", "https")

		if got := webAuthnRequestOrigin(req); got != "https://files.example.com" {
			t.Fatalf("webAuthnRequestOrigin() = %q, want https://files.example.com", got)
		}
	})

	t.Run("ignores untrusted forwarded headers", func(t *testing.T) {
		settings.Config.Http.TrustProxyHeaders = false
		req := httptest.NewRequest("GET", "http://127.0.0.1:8080/", nil)
		req.Host = "127.0.0.1:8080"
		req.Header.Set("X-Forwarded-Host", "evil.example.com")
		req.Header.Set("X-Forwarded-Proto", "https")

		if got := webAuthnRequestOrigin(req); got != "http://127.0.0.1:8080" {
			t.Fatalf("webAuthnRequestOrigin() = %q, want http://127.0.0.1:8080", got)
		}
	})
}
