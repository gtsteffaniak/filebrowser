package settings

import "testing"

func TestMigrateLegacyPasswordAdminFromAuth(t *testing.T) {
	saved := Config.Auth
	t.Cleanup(func() { Config.Auth = saved })

	Config.Auth = Auth{
		AdminUsername: "legacy-admin",
		AdminPassword: "legacy-pass",
		Methods: LoginMethods{
			PasswordAuth: PasswordAuthConfig{Enabled: true},
		},
	}

	MigrateLegacyPasswordAdminFromAuth()

	if Config.Auth.AdminUsername != "" || Config.Auth.AdminPassword != "" {
		t.Fatalf("expected deprecated auth fields cleared, got username=%q password=%q", Config.Auth.AdminUsername, Config.Auth.AdminPassword)
	}
	if Config.Auth.Methods.PasswordAuth.AdminUsername != "legacy-admin" {
		t.Fatalf("adminUsername = %q, want legacy-admin", Config.Auth.Methods.PasswordAuth.AdminUsername)
	}
	if Config.Auth.Methods.PasswordAuth.AdminPassword != "legacy-pass" {
		t.Fatalf("adminPassword = %q, want legacy-pass", Config.Auth.Methods.PasswordAuth.AdminPassword)
	}
}

func TestMigrateLegacyPasswordAdminFromAuth_DoesNotOverrideNewLocation(t *testing.T) {
	saved := Config.Auth
	t.Cleanup(func() { Config.Auth = saved })

	Config.Auth = Auth{
		AdminUsername: "legacy-admin",
		Methods: LoginMethods{
			PasswordAuth: PasswordAuthConfig{
				AdminUsername: "new-admin",
			},
		},
	}

	MigrateLegacyPasswordAdminFromAuth()

	if Config.Auth.Methods.PasswordAuth.AdminUsername != "new-admin" {
		t.Fatalf("adminUsername = %q, want new-admin", Config.Auth.Methods.PasswordAuth.AdminUsername)
	}
}

func TestPasswordAdminUsername_DefaultsToAdmin(t *testing.T) {
	saved := Config.Auth.Methods.PasswordAuth
	t.Cleanup(func() { Config.Auth.Methods.PasswordAuth = saved })
	Config.Auth.Methods.PasswordAuth.AdminUsername = ""

	if got := PasswordAdminUsername(); got != "admin" {
		t.Fatalf("PasswordAdminUsername() = %q, want admin", got)
	}
}
