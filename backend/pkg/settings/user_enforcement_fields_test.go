package settings

import (
	"errors"
	"testing"
)

func TestValidateSelfUserUpdateNotEnforced_blocksEnforcedField(t *testing.T) {
	t.Parallel()
	enforced := UserDefaultsEnforcement{
		Listing: UserDefaultsListingEnforcement{DateFormat: true},
	}
	err := ValidateSelfUserUpdateNotEnforced([]string{"dateFormat"}, enforced)
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
	if err := ValidateSelfUserUpdateNotEnforced([]string{"showHidden"}, enforced); err != nil {
		t.Fatalf("unexpected: %v", err)
	}
}
