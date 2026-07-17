package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
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

func TestAdminHasSharePermissionAfterSettingsMigration(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize(settingsMigrationConfigPath(t))
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	settings.Config.Server.DatabaseV2.Path = dbPath
	settings.Config.Server.DatabaseV2.MigrateFrom = settingsMigrationBoltPath(t)

	if err := migrateFromBoltToSQLite(); err != nil {
		t.Fatal(err)
	}

	_, err := state.Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() { _ = state.Close() })

	validateUserInfo(false)

	admin, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if !admin.Permissions.Share {
		t.Fatalf("admin Share=false after settings migration; perms=%+v", admin.Permissions)
	}
	assertSourceSidebarLinkCount(t, &admin, 3)
}

func assertSourceSidebarLinkCount(t *testing.T, user *users.User, want int) {
	t.Helper()
	count := 0
	for _, link := range user.SidebarLinks {
		if strings.HasPrefix(link.Category, "source") {
			count++
		}
	}
	if count != want {
		t.Fatalf("user %q has %d source sidebar links, want %d: %v",
			user.Username, count, want, user.SidebarLinks)
	}
}
