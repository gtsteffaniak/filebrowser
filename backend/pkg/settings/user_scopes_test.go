package settings_test

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestExpandBackendScopesForCreateUserDir(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	jwtConfig := "../../../_docker/src/jwt/backend/config.yaml"
	settings.Initialize(jwtConfig)
	settings.Env.IsPlaywright = true

	u := &users.User{
		FrontendUser: users.FrontendUser{Username: "testadmin"},
		BackendScopes: []users.BackendScope{
			{Path: settings.Config.Server.Sources[0].Path, Scope: "/"},
		},
	}
	settings.ExpandBackendScopesForCreateUserDir(u)
	if len(u.BackendScopes) != 1 {
		t.Fatalf("scopes: %+v", u.BackendScopes)
	}
	if u.BackendScopes[0].Scope != "/testadmin" {
		t.Fatalf("scope=%q want /testadmin", u.BackendScopes[0].Scope)
	}
}

func TestApplyUserDefaultsFrom_expandsCreateUserDirScope(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	jwtConfig := "../../../_docker/src/jwt/backend/config.yaml"
	settings.Initialize(jwtConfig)
	settings.Env.IsPlaywright = true

	u := &users.User{FrontendUser: users.FrontendUser{Username: "testuser"}}
	settings.ApplyUserDefaultsFrom(u, settings.Config.UserDefaults)
	if len(u.BackendScopes) != 1 {
		t.Fatalf("scopes: %+v", u.BackendScopes)
	}
	if u.BackendScopes[0].Scope != "/testuser" {
		t.Fatalf("scope=%q want /testuser", u.BackendScopes[0].Scope)
	}
}
