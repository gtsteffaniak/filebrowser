package state

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func TestProxyUserCreateHasSharePermission(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	proxyConfig := "../../_docker/src/proxy/backend/config.yaml"
	settings.Initialize(proxyConfig)
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

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
		t.Fatalf("Share=false after create+reload; perms=%+v defaults.share=%v",
			loaded.Permissions, settings.Config.UserDefaults.Account.Permissions.Share)
	}

	_ = Close()
	_, err = Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	reloaded, err := GetUserByUsername("demo-127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.Permissions.Share {
		t.Fatalf("Share=false after DB reload; perms=%+v", reloaded.Permissions)
	}
}
