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
	fberrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
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
	_, err := getOrCreateAuthenticatedUser("ldap-user", users.LoginMethodLdap, false, groups, true)
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
	_, err := getOrCreateAuthenticatedUser("ldap-user", users.LoginMethodLdap, false, groups, true)
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
	_, err := getOrCreateAuthenticatedUser("denied-ldap-user", users.LoginMethodLdap, false, groups, true)
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

// setupDefaultEnabledTestSource points source config at a temp dir so
// auto-created users receive a default-enabled backend scope.
func setupDefaultEnabledTestSource(t *testing.T) string {
	t.Helper()
	srcPath := t.TempDir()
	savedSources := settings.Config.Server.Sources
	savedSourceMap := settings.Config.Server.SourceMap
	savedNameToSource := settings.Config.Server.NameToSource
	t.Cleanup(func() {
		settings.Config.Server.Sources = savedSources
		settings.Config.Server.SourceMap = savedSourceMap
		settings.Config.Server.NameToSource = savedNameToSource
	})
	src := &settings.Source{
		Path:   srcPath,
		Name:   "src",
		Config: settings.SourceConfig{DefaultEnabled: true, DefaultUserScope: "/"},
	}
	settings.Config.Server.Sources = []*settings.Source{src}
	settings.Config.Server.SourceMap[srcPath] = src
	settings.Config.Server.NameToSource["src"] = src
	settings.InitializeUserResolvers()
	return srcPath
}

func TestGetOrCreateAuthenticatedUserDeniesSourceAccessToNewUsers(t *testing.T) {
	setupTestEnv(t)
	setupDefaultEnabledTestSource(t)

	user, err := getOrCreateAuthenticatedUser("newbie", users.LoginMethodOidc, false, nil)
	if err != nil {
		t.Fatalf("getOrCreateAuthenticatedUser() err = %v", err)
	}
	if user.Permissions.Admin {
		t.Fatalf("auto-created user should not be admin")
	}
	if len(user.BackendScopes) == 0 {
		t.Fatalf("auto-created user has no backend scopes, default source not applied")
	}
	denyAll := users.DenyAllSourceFilePermissions()
	for _, scope := range user.BackendScopes {
		if scope.Permissions != denyAll {
			t.Fatalf("scope %q permissions = %+v, want deny-all", scope.Path, scope.Permissions)
		}
	}

	loaded, err := state.GetUserByUsername("newbie")
	if err != nil {
		t.Fatalf("GetUserByUsername() err = %v", err)
	}
	perms, err := loaded.FilePermsForSourceName("src")
	if err != nil {
		t.Fatalf("FilePermsForSourceName() err = %v for new user", err)
	}
	if perms != denyAll {
		t.Fatalf("persisted source permissions = %+v, want deny-all", perms)
	}
}

func TestGetOrCreateAuthenticatedUserGrantsAdminsFullSourceAccess(t *testing.T) {
	setupTestEnv(t)
	setupDefaultEnabledTestSource(t)

	user, err := getOrCreateAuthenticatedUser("boss", users.LoginMethodOidc, true, nil)
	if err != nil {
		t.Fatalf("getOrCreateAuthenticatedUser() err = %v", err)
	}
	if !user.Permissions.Admin {
		t.Fatalf("auto-created admin user should have admin permission")
	}
	if len(user.BackendScopes) == 0 {
		t.Fatalf("auto-created admin user has no backend scopes")
	}
	full := settings.AdminSourceFilePermissions()
	for _, scope := range user.BackendScopes {
		if scope.Permissions != full {
			t.Fatalf("admin scope %q permissions = %+v, want full access", scope.Path, scope.Permissions)
		}
	}

	loaded, err := state.GetUserByUsername("boss")
	if err != nil {
		t.Fatalf("GetUserByUsername() err = %v", err)
	}
	perms, err := loaded.FilePermsForSourceName("src")
	if err != nil {
		t.Fatalf("FilePermsForSourceName() err = %v for admin user", err)
	}
	if perms != full {
		t.Fatalf("persisted admin source permissions = %+v, want full access", perms)
	}
}

func TestGetOrCreateAuthenticatedUserPromotesExistingUserToAdminAccess(t *testing.T) {
	setupTestEnv(t)
	setupDefaultEnabledTestSource(t)

	// First login: auto-created as a regular user (deny-all).
	if _, err := getOrCreateAuthenticatedUser("late", users.LoginMethodOidc, false, nil); err != nil {
		t.Fatalf("getOrCreateAuthenticatedUser() err = %v", err)
	}

	// Second login: user is now in the admin group.
	user, err := getOrCreateAuthenticatedUser("late", users.LoginMethodOidc, true, nil)
	if err != nil {
		t.Fatalf("getOrCreateAuthenticatedUser() err = %v", err)
	}
	if !user.Permissions.Admin {
		t.Fatalf("promoted user should have admin permission")
	}
	full := settings.AdminSourceFilePermissions()
	for _, scope := range user.BackendScopes {
		if scope.Permissions != full {
			t.Fatalf("promoted admin scope %q permissions = %+v, want full access", scope.Path, scope.Permissions)
		}
	}

	loaded, err := state.GetUserByUsername("late")
	if err != nil {
		t.Fatalf("GetUserByUsername() err = %v", err)
	}
	if !loaded.Permissions.Admin {
		t.Fatalf("promoted user was not persisted as admin")
	}
	perms, err := loaded.FilePermsForSourceName("src")
	if err != nil {
		t.Fatalf("FilePermsForSourceName() err = %v for promoted admin", err)
	}
	if perms != full {
		t.Fatalf("persisted promoted admin source permissions = %+v, want full access", perms)
	}
}
