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
