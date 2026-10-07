package settings

import "testing"

func TestValidatePasswordPolicy(t *testing.T) {
	orig := Config.Auth.Methods.PasswordAuth.MinLength
	defer func() { Config.Auth.Methods.PasswordAuth.MinLength = orig }()

	Config.Auth.Methods.PasswordAuth.MinLength = 8
	if err := ValidatePasswordPolicy("short"); err == nil {
		t.Fatal("expected error for short password")
	}
	if err := ValidatePasswordPolicy("longenough"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	Config.Auth.Methods.PasswordAuth.MinLength = 0
	if PasswordMinLength() != 5 {
		t.Fatalf("default min length = %d, want 5", PasswordMinLength())
	}
	if err := ValidatePasswordPolicy("1234"); err == nil {
		t.Fatal("expected default min length 5 to reject 4 chars")
	}
}
