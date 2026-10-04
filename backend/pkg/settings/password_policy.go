package settings

import "fmt"

// PasswordMinLength returns the configured minimum password length (default 5).
func PasswordMinLength() int {
	n := Config.Auth.Methods.PasswordAuth.MinLength
	if n < 1 {
		return 5
	}
	return n
}

// ValidatePasswordPolicy checks plaintext passwords against server auth.password settings.
func ValidatePasswordPolicy(password string) error {
	min := PasswordMinLength()
	if len(password) < min {
		return fmt.Errorf("password must be at least %d characters", min)
	}
	return nil
}
