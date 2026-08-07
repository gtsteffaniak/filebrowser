package state

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

func TestPrepShareValuesForFrontend_emptyReturnsJSONArray(t *testing.T) {
	got := PrepShareValuesForFrontend(nil, httptest.NewRequest("GET", "/", nil), "example.com", "https", nil)
	if got == nil {
		t.Fatal("expected non-nil slice for empty shares")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got len=%d", len(got))
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) != "[]" {
		t.Fatalf("expected JSON [], got %s", string(raw))
	}
}

func TestPrepShareValuesForFrontend_nonEmptyPreservesEntries(t *testing.T) {
	shares := []share.Share{{
		ShareColumns: share.ShareColumns{Hash: "abc123", Path: "/docs"},
	}}
	got := PrepShareValuesForFrontend(nil, httptest.NewRequest("GET", "/", nil), "example.com", "https", shares)
	if len(got) != 1 {
		t.Fatalf("expected 1 share, got %d", len(got))
	}
	if got[0].Hash != "abc123" {
		t.Fatalf("hash = %q, want abc123", got[0].Hash)
	}
}
