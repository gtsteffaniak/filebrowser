package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	fberrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestGetOrCreateAuthenticatedUserRejectsLoginMethodMismatch(t *testing.T) {
	setupTestEnv(t)

	passwordUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "graham",
			LoginMethod: users.LoginMethodPassword,
		},
	}
	if err := state.CreateUser(passwordUser, "secret"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err := getOrCreateAuthenticatedUser("graham", users.LoginMethodOidc, false, nil, false)
	if !errors.Is(err, fberrors.ErrWrongLoginMethod) {
		t.Fatalf("getOrCreateAuthenticatedUser() err = %v, want ErrWrongLoginMethod", err)
	}

	loaded, err := state.GetUserByUsername("graham")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if loaded.LoginMethod != users.LoginMethodPassword {
		t.Fatalf("loginMethod changed to %q, want password", loaded.LoginMethod)
	}
}

func TestGetOrCreateAuthenticatedUserRejectsPasswordUserForOIDCAccount(t *testing.T) {
	setupTestEnv(t)

	oidcUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "graham",
			LoginMethod: users.LoginMethodOidc,
		},
	}
	if err := state.CreateUser(oidcUser, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	_, err := getOrCreateAuthenticatedUser("graham", users.LoginMethodOidc, false, nil, false)
	if err != nil {
		t.Fatalf("getOrCreateAuthenticatedUser() for matching oidc user: %v", err)
	}
}

func TestGetOrCreateAuthenticatedUserLDAPUserGroupsCNMatch(t *testing.T) {
	setupTestEnv(t)

	orig := settings.Config.Auth.Methods.LdapAuth.UserGroups
	settings.Config.Auth.Methods.LdapAuth.UserGroups = []string{"employees"}
	t.Cleanup(func() {
		settings.Config.Auth.Methods.LdapAuth.UserGroups = orig
	})

	ldapUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "ldap-user",
			LoginMethod: users.LoginMethodLdap,
		},
	}
	if err := state.CreateUser(ldapUser, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	groups := []string{"cn=Employees,ou=groups,dc=example,dc=com"}
	_, err := getOrCreateAuthenticatedUser("ldap-user", users.LoginMethodLdap, false, groups)
	if err != nil {
		t.Fatalf("getOrCreateAuthenticatedUser() with CN-only userGroups: %v", err)
	}
}

func TestGetOrCreateAuthenticatedUserLDAPUserGroupsDenied(t *testing.T) {
	setupTestEnv(t)

	orig := settings.Config.Auth.Methods.LdapAuth.UserGroups
	settings.Config.Auth.Methods.LdapAuth.UserGroups = []string{"employees"}
	t.Cleanup(func() {
		settings.Config.Auth.Methods.LdapAuth.UserGroups = orig
	})

	ldapUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "ldap-user",
			LoginMethod: users.LoginMethodLdap,
		},
	}
	if err := state.CreateUser(ldapUser, ""); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	groups := []string{"cn=Contractors,ou=groups,dc=example,dc=com"}
	_, err := getOrCreateAuthenticatedUser("ldap-user", users.LoginMethodLdap, false, groups)
	if err == nil || err.Error() != "user is not in allowed groups" {
		t.Fatalf("getOrCreateAuthenticatedUser() err = %v, want user is not in allowed groups", err)
	}
}

func TestGetOrCreateAuthenticatedUserLDAPDeniedFirstLoginCreatesNoAccount(t *testing.T) {
	setupTestEnv(t)

	orig := settings.Config.Auth.Methods.LdapAuth.UserGroups
	settings.Config.Auth.Methods.LdapAuth.UserGroups = []string{"employees"}
	t.Cleanup(func() {
		settings.Config.Auth.Methods.LdapAuth.UserGroups = orig
	})

	groups := []string{"cn=Contractors,ou=groups,dc=example,dc=com"}
	_, err := getOrCreateAuthenticatedUser("denied-ldap-user", users.LoginMethodLdap, false, groups)
	if err == nil || err.Error() != "user is not in allowed groups" {
		t.Fatalf("getOrCreateAuthenticatedUser() err = %v, want user is not in allowed groups", err)
	}

	_, getErr := state.GetUserByUsername("denied-ldap-user")
	if !errors.Is(getErr, fberrors.ErrNotExist) {
		t.Fatalf("GetUserByUsername after denied login: %v, want ErrNotExist", getErr)
	}
}

func TestAuthenticatePasswordRejectsWrongLoginMethod(t *testing.T) {
	setupTestEnv(t)

	oidcUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "graham",
			LoginMethod: users.LoginMethodOidc,
		},
	}
	if err := state.CreateUser(oidcUser, "secret"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login?username=graham", nil)
	req.Header.Set("X-Password", url.QueryEscape("secret"))

	_, err := auth.AuthenticatePassword(req, true, false)
	if !errors.Is(err, fberrors.ErrWrongLoginMethod) {
		t.Fatalf("AuthenticatePassword() err = %v, want ErrWrongLoginMethod", err)
	}
}

func TestLoginHelperReturnsInvalidLoginMethodForPasswordMismatch(t *testing.T) {
	setupTestEnv(t)
	app.MustWireServices(state.Default())

	oidcUser := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "graham",
			LoginMethod: users.LoginMethodOidc,
		},
	}
	if err := state.CreateUser(oidcUser, "secret"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	origPassword := settings.Config.Auth.Methods.PasswordAuth.Enabled
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	t.Cleanup(func() {
		settings.Config.Auth.Methods.PasswordAuth.Enabled = origPassword
	})

	handler := LoginHelper(true, func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		return http.StatusOK, nil
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login?username=graham", nil)
	req.Header.Set("X-Password", url.QueryEscape("secret"))
	rec := httptest.NewRecorder()

	status, err := handler(rec, req, &requestContext{})
	if status != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", status)
	}
	if !errors.Is(err, fberrors.ErrInvalidLoginMethod) {
		t.Fatalf("err = %v, want ErrInvalidLoginMethod", err)
	}
}
