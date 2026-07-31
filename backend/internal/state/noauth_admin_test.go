package state

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestEnsureNoAuthAdminUserAfterMigration_leavesSingleUserUnchanged(t *testing.T) {
	settings.Config.Auth.Methods.NoAuth = true
	settings.Config.Auth.AdminUsername = "admin"
	t.Cleanup(func() {
		settings.Config.Auth.Methods.NoAuth = false
		settings.Config.Auth.AdminUsername = "admin"
	})

	store, _, err := sqldb.NewSQLStoreWithOptions(filepath.Join(t.TempDir(), "test.sqlite"), sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	id, err := utils.RandomUint64ID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser(&users.User{
		ID: id,
		FrontendUser: users.FrontendUser{
			Username:    "phil",
			Permissions: users.Permissions{Admin: true},
		},
	}); err != nil {
		t.Fatal(err)
	}

	if err := EnsureNoAuthAdminUserAfterMigration(store); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetUserByUsername("phil"); err != nil {
		t.Fatalf("expected phil unchanged: %v", err)
	}
	if _, err := store.GetUserByUsername("admin"); err == nil {
		t.Fatal("expected no admin user to be created for single-user migration")
	}
}

func TestEnsureNoAuthAdminUserAfterMigration_createsAdminWhenMultipleUsers(t *testing.T) {
	settings.Config.Auth.Methods.NoAuth = true
	settings.Config.Auth.AdminUsername = "admin"
	t.Cleanup(func() {
		settings.Config.Auth.Methods.NoAuth = false
		settings.Config.Auth.AdminUsername = "admin"
	})

	store, _, err := sqldb.NewSQLStoreWithOptions(filepath.Join(t.TempDir(), "test.sqlite"), sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	for _, name := range []string{"alice", "bob"} {
		id, err := utils.RandomUint64ID()
		if err != nil {
			t.Fatal(err)
		}
		if err := store.CreateUser(&users.User{
			ID:           id,
			FrontendUser: users.FrontendUser{Username: name},
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := EnsureNoAuthAdminUserAfterMigration(store); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetUserByUsername("admin"); err != nil {
		t.Fatalf("expected admin user to be created: %v", err)
	}
	if _, err := store.GetUserByUsername("alice"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetUserByUsername("bob"); err != nil {
		t.Fatal(err)
	}
}

func TestResolveNoAuthUser_usesSoleUserWhenAdminMissing(t *testing.T) {
	settings.Config.Auth.Methods.NoAuth = true
	settings.Config.Auth.AdminUsername = "admin"
	t.Cleanup(func() {
		settings.Config.Auth.Methods.NoAuth = false
		settings.Config.Auth.AdminUsername = "admin"
	})

	store, _, err := sqldb.NewSQLStoreWithOptions(filepath.Join(t.TempDir(), "test.sqlite"), sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	sqlDb = store

	id, err := utils.RandomUint64ID()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CreateUser(&users.User{
		ID: id,
		FrontendUser: users.FrontendUser{
			Username:    "phil",
			Permissions: users.Permissions{Admin: true},
		},
	}); err != nil {
		t.Fatal(err)
	}

	user, err := ResolveNoAuthUser()
	if err != nil {
		t.Fatal(err)
	}
	if user.Username != "phil" {
		t.Fatalf("username = %q, want phil", user.Username)
	}
}

func TestEnsureNoAuthAdminUserAfterMigration_skipsWhenNotNoauth(t *testing.T) {
	settings.Config.Auth.Methods.NoAuth = false
	t.Cleanup(func() { settings.Config.Auth.Methods.NoAuth = false })

	store, _, err := sqldb.NewSQLStoreWithOptions(filepath.Join(t.TempDir(), "test.sqlite"), sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	id, err := utils.RandomUint64ID()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CreateUser(&users.User{
		ID:           id,
		FrontendUser: users.FrontendUser{Username: "alice"},
	}); err != nil {
		t.Fatal(err)
	}
	id2, err := utils.RandomUint64ID()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateUser(&users.User{
		ID:           id2,
		FrontendUser: users.FrontendUser{Username: "bob"},
	}); err != nil {
		t.Fatal(err)
	}

	if err := EnsureNoAuthAdminUserAfterMigration(store); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetUserByUsername("admin"); err == nil {
		t.Fatal("expected no admin user when noauth is disabled")
	}
}
