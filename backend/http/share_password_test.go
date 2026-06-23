package http

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/database/share"
)

func TestApplySharePasswordUpdatePreservesWhenOmitted(t *testing.T) {
	link := &share.Share{
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
	link := &share.Share{
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
	link := &share.Share{
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
