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
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestValidateCustomShareHash(t *testing.T) {
	valid := []string{"a", "workfiles", "my-share_01", strings.Repeat("x", 64)}
	for _, h := range valid {
		if err := validateCustomShareHash(h); err != nil {
			t.Errorf("validateCustomShareHash(%q) unexpected error: %v", h, err)
		}
	}
	invalid := []string{"", "has space", "a/b", "a.b", "../x", "héllo", "a?b", "a#b", strings.Repeat("x", 65)}
	for _, h := range invalid {
		if err := validateCustomShareHash(h); err == nil {
			t.Errorf("validateCustomShareHash(%q) expected error", h)
		}
	}
}

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

func TestSharePostCustomHash_RejectedWhenDisabled(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	settings.Env.UnsafeCustomShare = false

	status, _ := postCreateWithCustomHash(t, &Context{User: owner}, "workfiles")
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 when UNSAFE_CUSTOM_SHARE is disabled, got %d", status)
	}
	if state.ShareHashExists("workfiles") {
		t.Fatalf("custom share must not be created when feature is disabled")
	}
}

func TestSharePostCustomHash_RejectsInvalidAndDuplicate(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	settings.Env.UnsafeCustomShare = true
	t.Cleanup(func() { settings.Env.UnsafeCustomShare = false })

	if status, _ := postCreateWithCustomHash(t, &Context{User: owner}, "bad/hash"); status != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid hash, got %d", status)
	}

	createVictimShare(t, owner.ID)
	status, _ := postCreateWithCustomHash(t, &Context{User: owner}, victimShareHash)
	if status != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate hash, got %d", status)
	}
}
