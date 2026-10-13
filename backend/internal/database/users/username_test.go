package users

import (
	"strings"
	"testing"
)

func TestValidateUsernameReserved(t *testing.T) {
	cases := []string{"anonymous", "Anonymous", " ANONYMOUS "}
	for _, username := range cases {
		if err := ValidateUsername(username); err == nil {
			t.Fatalf("expected reserved username error for %q", username)
		} else if !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("unexpected error for %q: %v", username, err)
		}
	}
}

func TestValidateUsernameAcceptsNormalName(t *testing.T) {
	if err := ValidateUsername("alice"); err != nil {
		t.Fatalf("expected valid username, got %v", err)
	}
}

func TestValidateUsernameRejectsUnsafeNames(t *testing.T) {
	for _, username := range []string{"..", "....", ".", "-", "al..ice", "bob/", "./bob", `bob\`, "alice/docs", " alice", "alice "} {
		if err := ValidateUsername(username); err == nil {
			t.Fatalf("expected %q to be rejected", username)
		}
	}
	for _, username := range []string{"alice", "first.last", "j.r.r", "a-b_c", "user@example.com", "José", "李雷", ".alice", "users"} {
		if err := ValidateUsername(username); err != nil {
			t.Fatalf("expected %q to be accepted, got %v", username, err)
		}
	}
}
