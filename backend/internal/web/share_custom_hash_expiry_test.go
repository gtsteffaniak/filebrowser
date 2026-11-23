package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

func TestSharePostCustomHash_AllowedWithExpiration(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	indexing.SetTestIndex("srv", "/srv")
	t.Cleanup(func() { indexing.ClearTestIndices() })

	owner.UseCustomShare = true

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
	if got.Alias != customHash {
		t.Fatalf("alias=%q want %q", got.Alias, customHash)
	}
	if got.Hash == customHash || got.Hash == "" {
		t.Fatalf("stored hash=%q must be a generated real hash, not the alias", got.Hash)
	}
	if presented := share.PresentedHash(&got); presented != customHash {
		t.Fatalf("presented hash=%q want alias %q", presented, customHash)
	}
	const hour = int64(time.Hour / time.Second)
	if got.Expire < before+hour || got.Expire > after+hour {
		t.Fatalf("Expire=%d want within [%d, %d]", got.Expire, before+hour, after+hour)
	}
}
