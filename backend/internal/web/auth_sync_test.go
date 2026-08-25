package web

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

type stubGroupSQL struct {
	saveErr error
}

func (s *stubGroupSQL) SaveAccessRule(string, string, *access.AccessRule) error { return nil }
func (s *stubGroupSQL) DeleteAccessRule(string, string) error                  { return nil }
func (s *stubGroupSQL) SaveGroup(string, access.StringSet) error               { return s.saveErr }
func (s *stubGroupSQL) DeleteGroup(string) error                               { return nil }
func (s *stubGroupSQL) SaveRevokedToken(string) error                          { return nil }
func (s *stubGroupSQL) SaveHashedToken(string, uint64) error                   { return nil }
func (s *stubGroupSQL) DeleteHashedToken(string) error                         { return nil }
func (s *stubGroupSQL) DeleteHashedTokensByUserID(uint64) error                { return nil }

func TestSetupJwtUser_OmittedGroupsClaimPreservesMembership(t *testing.T) {
	setupTestEnv(t)
	settings.Config.Auth.Methods.JwtAuth.GroupsClaim = "groups"
	settings.Config.Auth.Methods.JwtAuth.UserGroups = nil

	user := users.User{
		FrontendUser: users.FrontendUser{
			Username:    "jwt-user",
			LoginMethod: users.LoginMethodJwt,
		},
	}
	state.ApplyUserDefaults(&user)
	if err := state.CreateUser(&user, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := state.AddUserToGroup("developers", "jwt-user"); err != nil {
		t.Fatalf("seed group: %v", err)
	}

	req := httptestNewRequest(t)
	if _, err := SetupJwtUser(req, &Context{}, "jwt-user", map[string]interface{}{}); err != nil {
		t.Fatalf("SetupJwtUser: %v", err)
	}
	groups := state.GetUserGroups("jwt-user")
	if len(groups) != 1 || groups[0] != "developers" {
		t.Fatalf("omitted groups claim wiped membership: %#v", groups)
	}
}

func TestSetupJwtUser_EmptyGroupsClaimClearsMembership(t *testing.T) {
	setupTestEnv(t)
	settings.Config.Auth.Methods.JwtAuth.GroupsClaim = "groups"
	settings.Config.Auth.Methods.JwtAuth.UserGroups = nil

	user := users.User{
		FrontendUser: users.FrontendUser{
			Username:    "jwt-user",
			LoginMethod: users.LoginMethodJwt,
		},
	}
	state.ApplyUserDefaults(&user)
	if err := state.CreateUser(&user, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := state.AddUserToGroup("developers", "jwt-user"); err != nil {
		t.Fatalf("seed group: %v", err)
	}

	req := httptestNewRequest(t)
	if _, err := SetupJwtUser(req, &Context{}, "jwt-user", map[string]interface{}{"groups": []string{}}); err != nil {
		t.Fatalf("SetupJwtUser: %v", err)
	}
	groups := state.GetUserGroups("jwt-user")
	if len(groups) != 0 {
		t.Fatalf("empty groups claim should clear membership, got %#v", groups)
	}
}

func TestSetupJwtUser_FailsWhenGroupSyncFails(t *testing.T) {
	setupTestEnv(t)
	settings.Config.Auth.Methods.JwtAuth.GroupsClaim = "groups"
	settings.Config.Auth.Methods.JwtAuth.UserGroups = nil

	user := users.User{
		FrontendUser: users.FrontendUser{
			Username:    "jwt-user",
			LoginMethod: users.LoginMethodJwt,
		},
	}
	state.ApplyUserDefaults(&user)
	if err := state.CreateUser(&user, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := state.AddUserToGroup("developers", "jwt-user"); err != nil {
		t.Fatalf("seed group: %v", err)
	}

	state.SetAccessSQLStoreForTest(&stubGroupSQL{saveErr: errors.New("save failed")})

	req := httptestNewRequest(t)
	if _, err := SetupJwtUser(req, &Context{}, "jwt-user", map[string]interface{}{"groups": []string{}}); err == nil {
		t.Fatal("expected authentication to fail when group sync fails")
	}
	groups := state.GetUserGroups("jwt-user")
	if len(groups) != 1 || groups[0] != "developers" {
		t.Fatalf("expected membership preserved after failed sync, got %#v", groups)
	}
}

func TestSetupJwtUser_AdminRevokedWhenGroupRemoved(t *testing.T) {
	setupTestEnv(t)
	settings.Config.Auth.Methods.JwtAuth.GroupsClaim = "groups"
	settings.Config.Auth.Methods.JwtAuth.AdminGroup = "admins"
	settings.Config.Auth.Methods.JwtAuth.UserGroups = []string{"users", "admins"}
	t.Cleanup(func() {
		settings.Config.Auth.Methods.JwtAuth = settings.JwtAuthConfig{}
	})

	req := httptestNewRequest(t)
	user, err := SetupJwtUser(req, &Context{}, "jwt-downgrade", map[string]interface{}{
		"groups": []string{"admins"},
	})
	if err != nil {
		t.Fatalf("SetupJwtUser admin: %v", err)
	}
	if !user.Permissions.Admin {
		t.Fatal("expected admin when in admins group")
	}

	user, err = SetupJwtUser(req, &Context{}, "jwt-downgrade", map[string]interface{}{
		"groups": []string{"users"},
	})
	if err != nil {
		t.Fatalf("SetupJwtUser user: %v", err)
	}
	if user.Permissions.Admin {
		t.Fatal("expected admin revoked when admins group removed")
	}
}

func httptestNewRequest(t *testing.T) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "/", http.NoBody)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	return req
}
