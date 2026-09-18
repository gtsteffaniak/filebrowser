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

	_, err := auth.AuthenticatePassword(req, true)
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
