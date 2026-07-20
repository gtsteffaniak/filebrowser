package web

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestGetSchemeTrustedProto(t *testing.T) {
	settings.Config.Http.TrustedHeaders = map[string]bool{"x-forwarded-proto": true}
	t.Cleanup(func() { settings.Config.Http.TrustedHeaders = nil })

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := GetScheme(req); got != "https" {
		t.Fatalf("GetScheme = %q, want https", got)
	}
}

func TestGetSchemeIgnoresSpoofedProtoWhenUntrusted(t *testing.T) {
	settings.Config.Http.TrustedHeaders = nil

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := GetScheme(req); got != "http" {
		t.Fatalf("GetScheme = %q, want http", got)
	}
}

func TestRequestSchemeTLSFallback(t *testing.T) {
	settings.Config.Http.TrustedHeaders = nil

	req := httptest.NewRequest("GET", "https://example.com/", nil)
	req.TLS = &tls.ConnectionState{}
	if got := requestScheme(req); got != "https" {
		t.Fatalf("requestScheme = %q, want https", got)
	}
}

func TestRequestSchemeIgnoresInvalidProto(t *testing.T) {
	settings.Config.Http.TrustedHeaders = map[string]bool{"x-forwarded-proto": true}
	t.Cleanup(func() { settings.Config.Http.TrustedHeaders = nil })

	req := httptest.NewRequest("GET", "http://example.com/", nil)
	req.Header.Set("X-Forwarded-Proto", "javascript")
	if got := requestScheme(req); got != "http" {
		t.Fatalf("requestScheme = %q, want http", got)
	}
}
