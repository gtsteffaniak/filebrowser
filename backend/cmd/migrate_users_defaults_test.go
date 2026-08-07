package cmd

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// Simulates migrateUsers() steps for a Bolt user without backend scopes or source permissions.
func TestMigrateUsersFlow_appliesConfigDefaultsAndSourcePermissions(t *testing.T) {
	loadMigrateTestConfig(t, "../../_docker/src/jwt/backend/config.yaml")

	user := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "legacy-bolt-user",
			Permissions: users.Permissions{
				Download: true,
				Modify:   true,
				Create:   true,
				Share:    true,
			},
		},
		Version: 2,
	}

	prevVersion := user.Version
	if len(user.BackendScopes) == 0 {
		settings.ApplyUserDefaults(user)
	}
	if prevVersion < users.SourcePermissionsMigrationVersion {
		user.Version = prevVersion
		if !users.MigrateToSourcePermissions(user) {
			t.Fatal("expected source permission migration")
		}
	}
	user.Version = users.ProfileStorageVersion

	if !user.Permissions.Share {
		t.Fatal("global share should remain after migration")
	}
	if user.Permissions.Modify || user.Permissions.Download {
		t.Fatalf("legacy global file ops should be cleared: %+v", user.Permissions)
	}
	if len(user.BackendScopes) == 0 {
		t.Fatal("ApplyUserDefaults should assign default-enabled sources")
	}
	perms := user.BackendScopes[0].Permissions
	if !perms.View || !perms.Modify || !perms.Create || !perms.Download {
		t.Fatalf("scope permissions after migration: %+v", perms)
	}
	if user.BackendScopes[0].Scope != "/legacy-bolt-user" {
		t.Fatalf("createUserDir scope=%q", user.BackendScopes[0].Scope)
	}
}

func TestMigrateUsersFlow_existingScopeGetsLegacyGlobalFileOps(t *testing.T) {
	loadMigrateTestConfig(t, "../../_docker/src/proxy/backend/config.yaml")

	user := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "scoped",
			Permissions: users.Permissions{
				Download: true,
				Modify:   false,
				Create:   false,
				Share:    true,
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: settings.Config.Server.Sources[0].Path, Scope: "/"},
		},
		Version: 2,
	}

	users.MigrateToSourcePermissions(user)
	perms := user.BackendScopes[0].Permissions
	if !perms.Download || perms.Modify || perms.Create {
		t.Fatalf("expected legacy global copied to scope: %+v", perms)
	}
}

func loadMigrateTestConfig(t *testing.T, path string) {
	t.Helper()
	settings.Config = settings.SetDefaults(true)
	if err := settings.LoadConfigWithDefaultsForTest(path); err != nil {
		t.Fatal(err)
	}
}
