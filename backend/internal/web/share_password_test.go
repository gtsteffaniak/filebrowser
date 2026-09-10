package web

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

func TestApplySharePasswordUpdatePreservesWhenOmitted(t *testing.T) {
	link := &share.Share{
		PasswordHash: "hashed",
	}

	if err := applySharePasswordUpdate(link, nil, ""); err != nil {
		t.Fatal(err)
	}
	if link.PasswordHash != "hashed" {
		t.Fatalf("expected preserved password hash, got hash=%q", link.PasswordHash)
	}
}

func TestApplySharePasswordUpdateClearsWhenEmpty(t *testing.T) {
	link := &share.Share{
		PasswordHash: "hashed",
	}
	empty := ""

	if err := applySharePasswordUpdate(link, &empty, ""); err != nil {
		t.Fatal(err)
	}
	if link.PasswordHash != "" {
		t.Fatalf("expected cleared password hash, got hash=%q", link.PasswordHash)
	}
}

func TestApplySharePasswordUpdateReplacesWhenProvided(t *testing.T) {
	link := &share.Share{
		PasswordHash: "old",
	}
	next := "new-password"

	if err := applySharePasswordUpdate(link, &next, "newhash"); err != nil {
		t.Fatal(err)
	}
	if link.PasswordHash != "newhash" {
		t.Fatalf("expected replaced password hash, got hash=%q", link.PasswordHash)
	}
}
