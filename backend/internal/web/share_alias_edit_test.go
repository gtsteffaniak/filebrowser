package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

// postShareEditAlias sends an edit request the way the share editor does. A nil customHash omits
// the key (alias untouched); a non-nil value is sent as-is, including "" to clear the alias.
func postShareEditAlias(t *testing.T, user *Context, hash string, customHash *string) (int, error) {
	t.Helper()
	body := map[string]interface{}{
		"hash":   hash,
		"path":   "/",
		"source": "srv",
	}
	if customHash != nil {
		body["customHash"] = *customHash
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	return sharePostHandler(rec, req, user)
}

func TestSharePostUpdate_ChangeAndClearAlias(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	indexing.SetTestIndex("srv", "/srv")
	t.Cleanup(func() { indexing.ClearTestIndices() })
	owner.AllowCustomShareURL = true
	ctx := &Context{User: owner}

	const first = "edit-alias-first-custom-01"
	const second = "edit-alias-second-custom-02"

	if status, err := postCreateWithCustomHash(t, ctx, first); status != http.StatusOK {
		t.Fatalf("create: status=%d err=%v", status, err)
	}
	created, err := state.GetShare(first)
	if err != nil {
		t.Fatalf("GetShare(alias): %v", err)
	}
	randomHash := created.Hash

	// Edit the alias using the identifier the editor holds (the alias), as the UI does.
	changed := second
	if status, err := postShareEditAlias(t, ctx, first, &changed); status != http.StatusOK {
		t.Fatalf("change alias: status=%d err=%v", status, err)
	}
	if _, err := state.GetShare(first); err == nil {
		t.Fatal("old alias still resolves after edit")
	}
	if got, err := state.GetShare(second); err != nil || got.Hash != randomHash {
		t.Fatalf("new alias not resolving to the share: err=%v", err)
	}
	if got, err := state.GetShare(randomHash); err != nil || got.Alias != second {
		t.Fatalf("random hash should still resolve after alias change: err=%v", err)
	}

	// Omitting customHash keeps the alias.
	if status, err := postShareEditAlias(t, ctx, second, nil); status != http.StatusOK {
		t.Fatalf("update without customHash: status=%d err=%v", status, err)
	}
	if got, err := state.GetShare(second); err != nil || got.Alias != second {
		t.Fatalf("omitted customHash must keep alias: err=%v", err)
	}

	// An empty customHash clears the alias; the random hash becomes the presented identifier.
	empty := ""
	if status, err := postShareEditAlias(t, ctx, second, &empty); status != http.StatusOK {
		t.Fatalf("clear alias: status=%d err=%v", status, err)
	}
	if _, err := state.GetShare(second); err == nil {
		t.Fatal("cleared alias still resolves")
	}
	if got, err := state.GetShare(randomHash); err != nil || got.Alias != "" {
		t.Fatalf("random hash after clear: err=%v alias=%q", err, got.Alias)
	}
}

func TestSharePostUpdate_AliasChangeRequiresPermission(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	indexing.SetTestIndex("srv", "/srv")
	t.Cleanup(func() { indexing.ClearTestIndices() })
	owner.AllowCustomShareURL = true
	ctx := &Context{User: owner}

	const alias = "edit-perm-alias-custom-0001"
	if status, err := postCreateWithCustomHash(t, ctx, alias); status != http.StatusOK {
		t.Fatalf("create: status=%d err=%v", status, err)
	}

	owner.AllowCustomShareURL = false
	next := "edit-perm-alias-blocked-0002"
	status, err := postShareEditAlias(t, ctx, alias, &next)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 when allowCustomShareURL is disabled, got %d err=%v", status, err)
	}
	if got, err := state.GetShare(alias); err != nil || got.Alias != alias {
		t.Fatalf("alias changed despite rejection: err=%v", err)
	}
}
