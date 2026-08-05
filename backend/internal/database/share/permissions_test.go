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

func TestClampShareEditable(t *testing.T) {
	t.Parallel()
	ownerPerms := users.SourceFilePermissions{
		View: true, Download: false, Modify: false, Create: true, Delete: false,
	}
	editable := ShareEditable{}
	editable.AllowModify = true
	editable.AllowCreate = true
	editable.AllowDelete = true
	editable.DisableDownload = false
	editable.DisableFileViewer = false
	editable.EnableOnlyOffice = true

	ClampShareEditable(ownerPerms, &editable)

	if editable.AllowModify {
		t.Fatal("expected AllowModify clamped to false")
	}
	if !editable.DisableDownload {
		t.Fatal("expected DisableDownload forced true when owner lacks download")
	}
	if editable.AllowCreate != true {
		t.Fatal("expected AllowCreate to remain true")
	}
	if editable.AllowDelete {
		t.Fatal("expected AllowDelete clamped to false")
	}

	noView := users.SourceFilePermissions{View: false, Download: true, Modify: true, Create: true, Delete: true}
	editable2 := ShareEditable{FrontendShareInfo: FrontendShareInfo{EnableOnlyOffice: true}}
	ClampShareEditable(noView, &editable2)
	if !editable2.DisableFileViewer {
		t.Fatal("expected DisableFileViewer when owner lacks view")
	}
	if editable2.EnableOnlyOffice {
		t.Fatal("expected EnableOnlyOffice false when owner lacks view")
	}
}
