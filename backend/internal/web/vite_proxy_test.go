package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestViteProxyHandler_ProxiesToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/__vite/@vite/client" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("vite-client"))
	}))
	t.Cleanup(upstream.Close)

	prev := viteDevUpstream
	viteDevUpstream = upstream.URL
	t.Cleanup(func() { viteDevUpstream = prev })

	req := httptest.NewRequest(http.MethodGet, "/__vite/@vite/client", nil)
	rec := httptest.NewRecorder()
	viteProxyHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if rec.Body.String() != "vite-client" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestViteProxyHandler_DoesNotForwardCredentials(t *testing.T) {
	var gotCookie, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	prev := viteDevUpstream
	viteDevUpstream = upstream.URL
	t.Cleanup(func() { viteDevUpstream = prev })

	req := httptest.NewRequest(http.MethodGet, "/__vite/@vite/client", nil)
	req.Header.Set("Cookie", "session=secret")
	req.Header.Set("Authorization", "Bearer token")
	rec := httptest.NewRecorder()
	viteProxyHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotCookie != "" {
		t.Fatalf("Cookie forwarded to Vite upstream: %s", gotCookie)
	}
	if gotAuth != "" {
		t.Fatalf("Authorization forwarded to Vite upstream: %s", gotAuth)
	}
}

func TestViteProxyHandler_UpstreamUnavailable(t *testing.T) {
	prev := viteDevUpstream
	viteDevUpstream = "http://127.0.0.1:1"
	t.Cleanup(func() { viteDevUpstream = prev })

	req := httptest.NewRequest(http.MethodGet, "/__vite/@vite/client", nil)
	rec := httptest.NewRecorder()
	viteProxyHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if !strings.Contains(rec.Body.String(), "make dev") {
		t.Fatalf("expected helpful dev message, got: %s", rec.Body.String())
	}
}
