package web

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestEffectiveFilePermsUsesUserSourcePermsOnly(t *testing.T) {
	initStreamGrantTestSources(t)

	sourcePath := "/default"
	userPerms := users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
	d := &requestContext{
		User: testUserWithSourcePerms(sourcePath, userPerms),
	}

	got, err := effectiveFilePerms(d, "default")
	if err != nil {
		t.Fatalf("effectiveFilePerms: %v", err)
	}
	if got != userPerms {
		t.Fatalf("effectiveFilePerms = %+v, want %+v", got, userPerms)
	}
}

func testUserWithSourcePerms(sourcePath string, perms users.SourceFilePermissions) *users.User {
	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "alice"},
		BackendScopes: []users.BackendScope{
			{Path: sourcePath, Scope: "/", Permissions: perms},
		},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			sourcePath: perms,
		},
		Version: users.SourcePermissionsMigrationVersion,
	}
	return user
}
