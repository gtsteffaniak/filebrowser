package state

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestStateInit_proxyYAML_seedsDefaultsAndCreatesUser(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	t.Setenv("FILEBROWSER_PLAYWRIGHT_TEST", "true")
	settings.Initialize("../../../_docker/src/proxy/backend/config.yaml")

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	effective := EffectiveUserDefaults()
	if !effective.Account.Permissions.Share {
		t.Fatal("DB-seeded defaults missing share from proxy yaml")
	}
	access := GetSourceAccessDefaults()
	if !access.View || !access.Modify || !access.Create {
		t.Fatalf("source access defaults: %+v", access)
	}

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "demo-127.0.0.1",
			LoginMethod: users.LoginMethodProxy,
		},
	}
	settings.ApplyUserDefaults(u)
	if err = CreateUser(u, ""); err != nil {
		t.Fatal(err)
	}
	loaded, err := GetUserByUsername("demo-127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Permissions.Share {
		t.Fatal("Share not persisted")
	}
	if loaded.BackendScopes[0].Scope != "/demo-127.0.0.1" {
		t.Fatalf("scope=%q", loaded.BackendScopes[0].Scope)
	}
	p := loaded.BackendScopes[0].Permissions
	if !p.Modify || !p.Create {
		t.Fatalf("user scope file permissions: %+v", p)
	}
}

func TestStateInit_jwtYAML_organizedDefaultsOnNewUser(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	t.Setenv("FILEBROWSER_PLAYWRIGHT_TEST", "true")
	settings.Initialize("../../../_docker/src/jwt/backend/config.yaml")

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	effective := EffectiveUserDefaults()
	if !effective.Account.Permissions.Share {
		t.Fatal("share not in effective defaults")
	}
	access := GetSourceAccessDefaults()
	if !access.Modify || !access.Create || !access.Download {
		t.Fatalf("source defaultPermissions from jwt yaml: %+v", access)
	}

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "testuser",
			LoginMethod: users.LoginMethodJwt,
		},
	}
	settings.ApplyUserDefaults(u)
	if err = CreateUser(u, ""); err != nil {
		t.Fatal(err)
	}
	loaded, err := GetUserByUsername("testuser")
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.DarkMode || !loaded.Preview.Image {
		t.Fatalf("profile fields: darkMode=%v preview.image=%v", loaded.DarkMode, loaded.Preview.Image)
	}
	if loaded.BackendScopes[0].Permissions.Modify != access.Modify {
		t.Fatalf("scope perms=%+v access=%+v", loaded.BackendScopes[0].Permissions, access)
	}
}
