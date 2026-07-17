package users

import "testing"

func TestMigrateToSourcePermissions(t *testing.T) {
	user := &User{
		FrontendUser: FrontendUser{
			Permissions: Permissions{
				Download: true,
				Modify:   true,
				Delete:   false,
				Create:   true,
				Share:    true,
			},
		},
		BackendScopes: []BackendScope{{Path: "/data/a", Scope: "/"}},
		Version:       3,
	}
	if !MigrateToSourcePermissions(user) {
		t.Fatal("expected migration to modify user")
	}
	if user.Version != SourcePermissionsMigrationVersion {
		t.Fatalf("version = %d, want %d", user.Version, SourcePermissionsMigrationVersion)
	}
	perms := user.BackendScopes[0].Permissions
	if !perms.View || !perms.Download || !perms.Modify || perms.Delete || !perms.Create {
			t.Fatalf("unexpected perms: %+v", perms)
		}
	if user.Permissions.Download || user.Permissions.Modify || user.Permissions.Create {
		t.Fatalf("global file ops should be cleared: %+v", user.Permissions)
	}
	if !user.Permissions.Share {
		t.Fatal("global share should remain")
	}
	if MigrateToSourcePermissions(user) {
		t.Fatal("second migration should be no-op")
	}
}

func TestSeedSourcePermissionsForPath(t *testing.T) {
	user := &User{
		Version: SourcePermissionsMigrationVersion,
		BackendScopes: []BackendScope{
			{Path: "/data/a", Scope: "/", Permissions: SourceFilePermissions{View: true}},
		},
	}
	defaults := SourceFilePermissions{View: true, Download: true}
	if SeedSourcePermissionsForPath(user, "/data/a", defaults) {
		t.Fatal("should not overwrite existing perms")
	}
	user.BackendScopes = append(user.BackendScopes, BackendScope{Path: "/data/b", Scope: "/"})
	if !SeedSourcePermissionsForPath(user, "/data/b", defaults) {
		t.Fatal("expected seed for new source")
	}
	found := false
	for _, scope := range user.BackendScopes {
		if scope.Path == "/data/b" && scope.Permissions.Download {
			found = true
		}
	}
	if !found {
		t.Fatal("expected defaults applied on new scope path")
	}
}
