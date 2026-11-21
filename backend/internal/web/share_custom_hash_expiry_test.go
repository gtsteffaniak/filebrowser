package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestSharePostCustomHash_AllowedWithExpiration(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	indexing.SetTestIndex("srv", "/srv")
	t.Cleanup(func() { indexing.ClearTestIndices() })

	prevUnsafe := settings.Env.UnsafeCustomShare
	settings.Env.UnsafeCustomShare = true
	t.Cleanup(func() { settings.Env.UnsafeCustomShare = prevUnsafe })

	const customHash = "workfiles-q3-review-2026"
	raw, err := json.Marshal(map[string]interface{}{
		"source":     "srv",
		"path":       "/",
		"customHash": customHash,
		"expires":    "1",
		"unit":       "hours",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	before := time.Now().Unix()
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	status, err := sharePostHandler(rec, req, &Context{User: owner})
	after := time.Now().Unix()
	if status != http.StatusOK {
		t.Fatalf("expected 200, got status=%d err=%v", status, err)
	}

	got, err := state.GetShare(customHash)
	if err != nil {
		t.Fatalf("GetShare(%q): %v", customHash, err)
	}
	if got.Hash != customHash {
		t.Fatalf("stored hash=%q want %q", got.Hash, customHash)
	}
	const hour = int64(time.Hour / time.Second)
	if got.Expire < before+hour || got.Expire > after+hour {
		t.Fatalf("Expire=%d want within [%d, %d]", got.Expire, before+hour, after+hour)
	}
}
