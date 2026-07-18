package share

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func initPermissionsTestSources(t *testing.T) {
	t.Helper()
	users.SetSourceNameResolver(func(name string) (string, error) {
		if name == "default" {
			return "/default", nil
		}
		return "", nil
	})
}

func TestEffectiveFilePermissionsUsesUserSourcePerms(t *testing.T) {
	t.Parallel()
	initPermissionsTestSources(t)

	userPerms := users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "alice"},
		BackendScopes: []users.BackendScope{
			{Path: "/default", Scope: "/", Permissions: userPerms},
		},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			"/default": userPerms,
		},
		Version: users.SourcePermissionsMigrationVersion,
	}

	got, err := EffectiveFilePermissions(user, nil, "default")
	if err != nil {
		t.Fatalf("EffectiveFilePermissions: %v", err)
	}
	if got != userPerms {
		t.Fatalf("EffectiveFilePermissions = %+v, want %+v", got, userPerms)
	}
}

func TestShareFilePermissions(t *testing.T) {
	t.Parallel()
	link := Share{ShareColumns: ShareColumns{Hash: "abc"}}
	link.DisableFileViewer = true
	link.DisableDownload = true
	link.AllowModify = true
	link.AllowDelete = false
	link.AllowCreate = true
	got := link.FilePermissions()
	want := users.SourceFilePermissions{
		View: false, Download: false, Modify: true, Delete: false, Create: true,
	}
	if got != want {
		t.Fatalf("FilePermissions = %+v, want %+v", got, want)
	}
}
