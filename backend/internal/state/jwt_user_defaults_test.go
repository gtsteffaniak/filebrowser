package state

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestJwtAutoCreateUserPersistsCreateUserDirScope(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	jwtConfig := "../../../_docker/src/jwt/backend/config.yaml"
	settings.Initialize(jwtConfig)
	settings.Env.IsPlaywright = true

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "testadmin",
			LoginMethod: users.LoginMethodJwt,
		},
	}
	settings.ApplyUserDefaults(u)
	if err = CreateUser(u, ""); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetUserByUsername("testadmin")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.BackendScopes) != 1 {
		t.Fatalf("scopes: %+v", loaded.BackendScopes)
	}
	if loaded.BackendScopes[0].Scope != "/testadmin" {
		t.Fatalf("scope=%q want /testadmin after reload", loaded.BackendScopes[0].Scope)
	}
}
