package settings

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestValidateSelfUserUpdateNotEnforced_blocksEnforcedField(t *testing.T) {
	t.Parallel()
	enforced := UserDefaultsEnforcement{
		Listing: UserDefaultsListingEnforcement{DateFormat: true},
	}
	regular := &users.User{FrontendUser: users.FrontendUser{Username: "alice"}}
	err := ValidateSelfUserUpdateNotEnforced([]string{"dateFormat"}, enforced, regular)
	if err == nil {
		t.Fatal("expected error for enforced dateFormat")
	}
	var locked ErrEnforcedUserField
	if !errors.As(err, &locked) {
		t.Fatalf("expected ErrEnforcedUserField, got %T", err)
	}
	if locked.Field != "dateFormat" {
		t.Fatalf("field: got %q", locked.Field)
	}
}

func TestValidateSelfUserUpdateNotEnforced_allowsNonEnforced(t *testing.T) {
	t.Parallel()
	enforced := UserDefaultsEnforcement{
		Listing: UserDefaultsListingEnforcement{DateFormat: true},
	}
	regular := &users.User{FrontendUser: users.FrontendUser{Username: "alice"}}
	if err := ValidateSelfUserUpdateNotEnforced([]string{"showHidden"}, enforced, regular); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}

func TestValidateSelfUserUpdateNotEnforced_skipsAdmin(t *testing.T) {
	t.Parallel()
	enforced := UserDefaultsEnforcement{
		Listing: UserDefaultsListingEnforcement{DateFormat: true},
	}
	admin := &users.User{FrontendUser: users.FrontendUser{Username: "admin"}}
	admin.Permissions.Admin = true
	if err := ValidateSelfUserUpdateNotEnforced([]string{"dateFormat"}, enforced, admin); err != nil {
		t.Fatalf("admin should bypass enforcement validation: %v", err)
	}
}
