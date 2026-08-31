package cmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupCLIUserTest(t *testing.T) {
	t.Helper()
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.InitializeCLI("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	store, _, err := state.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(store)
	t.Cleanup(func() { _ = state.Close() })
}

func TestSetUserConvertsOIDCToPassword(t *testing.T) {
	setupCLIUserTest(t)

	oidcUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "graham",
			LoginMethod: users.LoginMethodOidc,
		},
	}
	if err := state.CreateUser(oidcUser, ""); err != nil {
		t.Fatal(err)
	}

	if err := setUser("graham", "newpassword", false); err != nil {
		t.Fatalf("setUser: %v", err)
	}

	loaded, err := state.GetUserByUsername("graham")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.LoginMethod != users.LoginMethodPassword {
		t.Fatalf("loginMethod = %q, want password", loaded.LoginMethod)
	}
}

func TestPromoteUserRejectsNonPasswordLoginMethod(t *testing.T) {
	setupCLIUserTest(t)

	oidcUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "graham",
			LoginMethod: users.LoginMethodOidc,
		},
	}
	if err := state.CreateUser(oidcUser, ""); err != nil {
		t.Fatal(err)
	}

	err := promoteUser("graham")
	if err == nil {
		t.Fatal("promoteUser: expected error for oidc user")
	}
	if !strings.Contains(err.Error(), "password-login") {
		t.Fatalf("promoteUser err = %q, want password-login restriction", err.Error())
	}
}

func TestPromoteUserAllowsPasswordLoginMethod(t *testing.T) {
	setupCLIUserTest(t)

	passwordUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "graham",
			LoginMethod: users.LoginMethodPassword,
		},
	}
	if err := state.CreateUser(passwordUser, "secret"); err != nil {
		t.Fatal(err)
	}

	if err := promoteUser("graham"); err != nil {
		t.Fatalf("promoteUser: %v", err)
	}

	loaded, err := state.GetUserByUsername("graham")
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Permissions.Admin {
		t.Fatal("expected admin after promote")
	}
}
