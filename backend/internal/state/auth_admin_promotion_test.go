package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestAuthAdminPromotionWithConfigUserDefaults(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	configPath := writeAuthAdminTestConfig(t)
	settings.Initialize(configPath)
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	if !settings.Env.ConfigUserDefaultsSpecified {
		t.Fatal("expected config userDefaults paths to be tracked")
	}
	if enforced := GetEnforcedUserDefaults(); enforced != (settings.UserDefaultsEnforcement{}) {
		t.Fatalf("expected empty enforcement on seed, got %+v", enforced)
	}

	for _, tc := range []struct {
		name        string
		username    string
		loginMethod users.LoginMethod
	}{
		{name: "oidc", username: "johndoe", loginMethod: users.LoginMethodOidc},
		{name: "jwt", username: "testadmin", loginMethod: users.LoginMethodJwt},
		{name: "ldap", username: "ldapadmin", loginMethod: users.LoginMethodLdap},
		{name: "proxy", username: "proxyadmin", loginMethod: users.LoginMethodProxy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			promoteAuthAdmin(t, tc.username, tc.loginMethod)
		})
	}
}

func TestAuthAdminPromotionWithEnforcedAdminDefault(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := `server:
  sources:
    - path: "../frontend/tests/playwright-files"
      config:
        defaultEnabled: true
        createUserDir: true
auth:
  methods:
    password:
      enabled: true
userDefaults:
  ui:
    darkMode: true
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	settings.Initialize(configPath)
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	if err := PatchUserDefaultsEnforced([]byte(`{"account":{"permissions":{"admin":true}}}`)); err != nil {
		t.Fatal(err)
	}

	promoteAuthAdmin(t, "jwt-admin", users.LoginMethodJwt)
}

func writeAuthAdminTestConfig(t *testing.T) string {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configYAML := `server:
  sources:
    - path: "../frontend/tests/playwright-files"
      config:
        defaultEnabled: true
        createUserDir: true
auth:
  methods:
    password:
      enabled: true
userDefaults:
  preview:
    image: true
  ui:
    darkMode: true
  listing:
    singleClick: false
  account:
    permissions:
      admin: false
      share: false
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	return configPath
}

func promoteAuthAdmin(t *testing.T, username string, loginMethod users.LoginMethod) {
	t.Helper()

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    username,
			LoginMethod: loginMethod,
		},
	}
	settings.ApplyUserDefaults(u)
	if err := CreateUser(u, ""); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Permissions.Admin {
		t.Fatal("expected non-admin user before auth admin sync")
	}

	loaded.Permissions.Admin = true
	if err = UpdateUser(&loaded, ""); err != nil {
		t.Fatalf("auth admin promotion failed: %v", err)
	}

	reloaded, err := GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Permissions.Admin {
		t.Fatal("expected admin after auth admin sync")
	}
}
