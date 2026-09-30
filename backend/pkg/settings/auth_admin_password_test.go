package settings

import "testing"

func TestConfigTriggersAdminPasswordReset(t *testing.T) {
	origNested := Config.Auth.Methods.PasswordAuth.AdminPassword
	origLegacy := Config.Auth.AdminPassword
	defer func() {
		Config.Auth.Methods.PasswordAuth.AdminPassword = origNested
		Config.Auth.AdminPassword = origLegacy
	}()

	Config.Auth.Methods.PasswordAuth.AdminPassword = ""
	Config.Auth.AdminPassword = ""
	if ConfigTriggersAdminPasswordReset() {
		t.Fatal("empty adminPassword should not trigger reset")
	}
	Config.Auth.AdminPassword = "admin"
	if ConfigTriggersAdminPasswordReset() {
		t.Fatal("admin adminPassword should not trigger reset")
	}
	Config.Auth.AdminPassword = "s3cret"
	if !ConfigTriggersAdminPasswordReset() {
		t.Fatal("non-default top-level adminPassword should trigger reset")
	}
	Config.Auth.Methods.PasswordAuth.AdminPassword = "nested-secret"
	Config.Auth.AdminPassword = "legacy-secret"
	if !ConfigTriggersAdminPasswordReset() {
		t.Fatal("nested adminPassword should trigger reset")
	}
	if got := PasswordAdminPassword(); got != "nested-secret" {
		t.Fatalf("PasswordAdminPassword() = %q, want nested-secret", got)
	}
}
