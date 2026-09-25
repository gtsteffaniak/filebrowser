package settings

import "testing"

func TestConfigTriggersAdminPasswordReset(t *testing.T) {
	orig := Config.Auth.AdminPassword
	defer func() { Config.Auth.AdminPassword = orig }()

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
		t.Fatal("non-default adminPassword should trigger reset")
	}
}
