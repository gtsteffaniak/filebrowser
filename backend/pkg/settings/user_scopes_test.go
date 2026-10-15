package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestExpandBackendScopesForCreateUserDirNeverKeepsDefaultScope(t *testing.T) {
	saved := Config
	defer func() { Config = saved }()
	Config = Settings{Server: Server{SourceMap: map[string]*Source{
		"/srv": {Path: "/srv", Name: "main", Config: SourceConfig{CreateUserDir: true, DefaultUserScope: "/users"}},
	}}}
	cases := map[string]string{"bob": "/users/bob", "users": "/users/users", "first.last": "/users/first.last"}
	for name, want := range cases {
		u := &users.User{
			FrontendUser:  users.FrontendUser{Username: name},
			BackendScopes: []users.BackendScope{{Path: "/srv", Scope: "/users"}},
		}
		ExpandBackendScopesForCreateUserDir(u)
		ExpandBackendScopesForCreateUserDir(u) // runs more than once per new user; must be idempotent
		if got := u.BackendScopes[0].Scope; got != want {
			t.Fatalf("%s: scope=%q want %q", name, got, want)
		}
	}
	// An admin-assigned scope that already ends in the username is kept.
	u := &users.User{
		FrontendUser:  users.FrontendUser{Username: "bob"},
		BackendScopes: []users.BackendScope{{Path: "/srv", Scope: "/users/bob"}},
	}
	ExpandBackendScopesForCreateUserDir(u)
	if got := u.BackendScopes[0].Scope; got != "/users/bob" {
		t.Fatalf("explicit scope changed: %q", got)
	}
}
