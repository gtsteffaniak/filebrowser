package cmd

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

func TestAdminHasSharePermissionAfterPlaywrightStartup(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	sharingConfig := "../../_docker/src/sharing/backend/config.yaml"
	settings.Initialize(sharingConfig)
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := state.Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() { _ = state.Close() })

	validateUserInfo(true)

	admin, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if !admin.Permissions.Share {
		t.Fatalf("admin Share=false after validateUserInfo; perms=%+v", admin.Permissions)
	}
	if !admin.Permissions.Admin {
		t.Fatalf("admin Admin=false; perms=%+v", admin.Permissions)
	}

	_ = state.Close()
	_, err = state.Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	admin, err = state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if !admin.Permissions.Share {
		t.Fatalf("admin Share=false after DB reload; perms=%+v", admin.Permissions)
	}
}
