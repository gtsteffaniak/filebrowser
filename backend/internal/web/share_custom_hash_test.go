package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

func postCreateWithCustomHash(t *testing.T, user *Context, customHash string) (int, error) {
	t.Helper()
	raw, err := json.Marshal(share.SharePostBody{
		ShareEditable: share.ShareEditable{
			ShareLimits: share.ShareLimits{SourceName: "srv"},
		},
		Path:       "/",
		CustomHash: customHash,
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	return sharePostHandler(rec, req, user)
}

func TestValidateCustomShareHashCharset(t *testing.T) {
	invalid := []string{"has space", "a/b", "a.b", "../x-with-enough-length", "héllo-with-long-name", "a?b-with-long-name"}
	for _, h := range invalid {
		if err := validateCustomShareHash(h); err == nil {
			t.Errorf("validateCustomShareHash(%q) expected error", h)
		}
	}
	if err := validateCustomShareHash(strings.Repeat("x", 16)); err != nil {
		t.Errorf("16-character hash should be valid: %v", err)
	}
}

func TestSharePostCustomHash_RejectedWhenUserDisabled(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	owner.AllowCustomShareURL = false

	status, err := postCreateWithCustomHash(t, &Context{User: owner}, "workfiles-q3-review-2026")
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 when allowCustomShareURL is disabled for the user, got %d", status)
	}
	if err == nil || !strings.Contains(err.Error(), "not enabled for your account") {
		t.Fatalf("expected account-level error message, got %v", err)
	}
	if state.ShareHashExists("workfiles-q3-review-2026") {
		t.Fatalf("custom share must not be created when disabled for the user")
	}
}

func TestSharePostCustomHash_RejectsInvalidAndDuplicate(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	owner.AllowCustomShareURL = true

	if status, _ := postCreateWithCustomHash(t, &Context{User: owner}, "bad/hash-with-enough-length"); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid hash, got %d", status)
	}

	createVictimShare(t, owner.ID)
	status, _ := postCreateWithCustomHash(t, &Context{User: owner}, victimShareHash)
	if status != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate hash, got %d", status)
	}
}
