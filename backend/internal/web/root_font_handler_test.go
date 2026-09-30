package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestRootFontHandler_ValidFont(t *testing.T) {
	prev := assetFs
	assetFs = fstest.MapFS{
		"fonts/roboto.woff2": &fstest.MapFile{Data: []byte("woff2-data")},
	}
	t.Cleanup(func() { assetFs = prev })

	req := httptest.NewRequest(http.MethodGet, "/fonts/roboto.woff2", nil)
	rec := httptest.NewRecorder()
	rootFontHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body, err := io.ReadAll(rec.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "woff2-data" {
		t.Fatalf("body = %q, want %q", body, "woff2-data")
	}
	if rec.Header().Get("Content-Type") != "font/woff2" {
		t.Fatalf("Content-Type = %q, want font/woff2", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "public, max-age=86400" {
		t.Fatalf("unexpected Cache-Control: %s", rec.Header().Get("Cache-Control"))
	}
}

func TestRootFontHandler_MissingFont(t *testing.T) {
	prev := assetFs
	assetFs = fstest.MapFS{}
	t.Cleanup(func() { assetFs = prev })

	req := httptest.NewRequest(http.MethodGet, "/fonts/missing.woff2", nil)
	rec := httptest.NewRecorder()
	rootFontHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestRootFontHandler_TraversalPath(t *testing.T) {
	prev := assetFs
	assetFs = fstest.MapFS{
		"fonts/secret.woff2": &fstest.MapFile{Data: []byte("secret")},
	}
	t.Cleanup(func() { assetFs = prev })

	req := httptest.NewRequest(http.MethodGet, "/fonts/../secret.woff2", nil)
	rec := httptest.NewRecorder()
	rootFontHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
