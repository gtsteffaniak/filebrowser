package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/database/share"
)

func TestApplySharePasswordUpdatePreservesWhenOmitted(t *testing.T) {
	link := &share.Link{
		PasswordHash: "hashed",
		Token:        "tok",
	}

	if err := applySharePasswordUpdate(link, nil, "", ""); err != nil {
		t.Fatal(err)
	}
	if link.PasswordHash != "hashed" || link.Token != "tok" {
		t.Fatalf("expected preserved secrets, got hash=%q token=%q", link.PasswordHash, link.Token)
	}
}

func TestApplySharePasswordUpdateClearsWhenEmpty(t *testing.T) {
	link := &share.Link{
		PasswordHash: "hashed",
		Token:        "tok",
	}
	empty := ""

	if err := applySharePasswordUpdate(link, &empty, "", ""); err != nil {
		t.Fatal(err)
	}
	if link.PasswordHash != "" || link.Token != "" {
		t.Fatalf("expected cleared secrets, got hash=%q token=%q", link.PasswordHash, link.Token)
	}
}

func TestApplySharePasswordUpdateReplacesWhenProvided(t *testing.T) {
	link := &share.Link{
		PasswordHash: "old",
		Token:        "oldtok",
	}
	next := "new-password"

	if err := applySharePasswordUpdate(link, &next, "newhash", "newtok"); err != nil {
		t.Fatal(err)
	}
	if link.PasswordHash != "newhash" || link.Token != "newtok" {
		t.Fatalf("expected replaced secrets, got hash=%q token=%q", link.PasswordHash, link.Token)
	}
}

func TestSharePostUpdate_PreservesPasswordWhenOmitted(t *testing.T) {
	owner, _, _ := setupShareAuthUsers(t)

	const hash = "password_preserve_hash"
	link := &share.Link{
		Hash:         hash,
		UserID:       owner.ID,
		Version:      1,
		PasswordHash: "existing-hash",
		Token:        "existing-token",
		CommonShare: share.CommonShare{
			Path:      "/",
			Source:    "/srv",
			ShareType: "normal",
			Title:     "before",
		},
	}
	if err := store.Share.Save(link); err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(map[string]interface{}{
		"hash":  hash,
		"title": "after",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	status, handlerErr := sharePostHandler(rec, req, &requestContext{user: owner})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got status=%d err=%v", status, handlerErr)
	}

	got, err := store.Share.GetByHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.PasswordHash != "existing-hash" {
		t.Fatalf("PasswordHash = %q, want existing-hash", got.PasswordHash)
	}
	if got.Token != "existing-token" {
		t.Fatalf("Token = %q, want existing-token", got.Token)
	}
	if got.Title != "after" {
		t.Fatalf("Title = %q, want after", got.Title)
	}
}

func TestSharePostUpdate_ClearsPasswordWhenEmptyString(t *testing.T) {
	owner, _, _ := setupShareAuthUsers(t)

	const hash = "password_clear_hash"
	link := &share.Link{
		Hash:         hash,
		UserID:       owner.ID,
		Version:      1,
		PasswordHash: "existing-hash",
		Token:        "existing-token",
		CommonShare: share.CommonShare{
			Path:      "/",
			Source:    "/srv",
			ShareType: "normal",
		},
	}
	if err := store.Share.Save(link); err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(map[string]interface{}{
		"hash":     hash,
		"password": "",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	status, handlerErr := sharePostHandler(rec, req, &requestContext{user: owner})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got status=%d err=%v", status, handlerErr)
	}

	got, err := store.Share.GetByHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.PasswordHash != "" || got.Token != "" {
		t.Fatalf("expected cleared password, got hash=%q token=%q", got.PasswordHash, got.Token)
	}
}
