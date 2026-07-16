package users

import "testing"

func TestEnsureSourcePermissionsForScopes_seedsMissingPaths(t *testing.T) {
	user := &User{
		Version: SourcePermissionsMigrationVersion,
		BackendScopes: []BackendScope{
			{Path: "/data/a", Scope: "/"},
			{Path: "/data/b", Scope: "/"},
		},
	}
	defaults := SourceFilePermissions{View: true, Download: true}
	adminDefaults := AdminSourceFilePermissionsForTest()

	if !EnsureSourcePermissionsForScopes(user, defaults, adminDefaults) {
		t.Fatal("expected repair to seed missing permissions")
	}
	if !user.BackendSourcePermissions["/data/a"].View {
		t.Fatal("expected view on /data/a")
	}
	if !user.BackendSourcePermissions["/data/b"].View {
		t.Fatal("expected view on /data/b")
	}
}

func TestEnsureSourcePermissionsForScopes_rekeysSourceName(t *testing.T) {
	had := SourceConfigLoaded()
	t.Cleanup(func() {
		if !had {
			SetSourceConfig(nil)
		}
	})
	SetSourceConfig(&SourceConfigProvider{
		GetSourceByPath: func(path string) (SourceInfo, bool) {
			if path == "/Users/dl" {
				return SourceInfo{Path: path, Name: "Downloads"}, true
			}
			return SourceInfo{}, false
		},
		GetSourceByName: func(name string) (SourceInfo, bool) {
			if name == "Downloads" {
				return SourceInfo{Path: "/Users/dl", Name: "Downloads"}, true
			}
			return SourceInfo{}, false
		},
	})

	user := &User{
		Version: SourcePermissionsMigrationVersion,
		BackendScopes: []BackendScope{
			{Path: "/Users/dl", Scope: "/"},
		},
		BackendSourcePermissions: map[string]SourceFilePermissions{
			"Downloads": {View: true, Download: true, Modify: true},
		},
	}
	defaults := SourceFilePermissions{View: true}
	adminDefaults := AdminSourceFilePermissionsForTest()

	if !EnsureSourcePermissionsForScopes(user, defaults, adminDefaults) {
		t.Fatal("expected rekey to change user")
	}
	perms := user.BackendScopes[0].Permissions
	if !perms.Modify {
		t.Fatal("expected modify preserved after rekey")
	}
	if _, stale := user.BackendSourcePermissions["Downloads"]; stale {
		t.Fatal("expected stale name key removed")
	}
}

func TestEnsureSourcePermissionsForScopes_adminGetsFullPerms(t *testing.T) {
	user := &User{
		Version: SourcePermissionsMigrationVersion,
		FrontendUser: FrontendUser{
			Permissions: Permissions{Admin: true},
		},
		BackendScopes: []BackendScope{
			{Path: "/data/a", Scope: "/"},
		},
	}
	defaults := SourceFilePermissions{View: true, Download: false, Modify: false}
	adminDefaults := AdminSourceFilePermissionsForTest()

	EnsureSourcePermissionsForScopes(user, defaults, adminDefaults)
	perms := user.BackendSourcePermissions["/data/a"]
	if !perms.View || !perms.Download || !perms.Modify || !perms.Create || !perms.Delete {
		t.Fatalf("admin seed = %+v, want full perms", perms)
	}
}

func AdminSourceFilePermissionsForTest() SourceFilePermissions {
	return SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
}
