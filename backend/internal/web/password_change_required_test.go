package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestLoginBlocksRequirePasswordChange(t *testing.T) {
	setupTestEnv(t)
	origPasswordAuth := settings.Config.Auth.Methods.PasswordAuth.Enabled
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	t.Cleanup(func() { settings.Config.Auth.Methods.PasswordAuth.Enabled = origPasswordAuth })

	user := &users.User{
		FrontendUser: users.FrontendUser{
			Username:              "mustchange",
			LoginMethod:           users.LoginMethodPassword,
			RequirePasswordChange: true,
		},
	}
	if err := state.CreateUser(user, "tempPass"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	handler := wrapHandler(loginHelper(loginHandler))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login?username=mustchange", nil)
	req.Header.Set("X-Password", "tempPass")
	handler(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("login status = %d, want 403", rec.Code)
	}
}

func TestChangeRequiredPasswordHandler(t *testing.T) {
	setupTestEnv(t)
	origPasswordAuth := settings.Config.Auth.Methods.PasswordAuth.Enabled
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	t.Cleanup(func() { settings.Config.Auth.Methods.PasswordAuth.Enabled = origPasswordAuth })

	user := &users.User{
		FrontendUser: users.FrontendUser{
			Username:              "mustchange2",
			LoginMethod:           users.LoginMethodPassword,
			RequirePasswordChange: true,
		},
	}
	if err := state.CreateUser(user, "tempPass"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"password":        "newSecurePass",
		"passwordConfirm": "newSecurePass",
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/password/change-required?username=mustchange2", bytes.NewReader(body))
	req.Header.Set("X-Password", "tempPass")
	req.Header.Set("Content-Type", "application/json")

	handler := wrapHandler(withoutUserHelper(changeRequiredPasswordHandler))
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("change-required status = %d body = %s", rec.Code, rec.Body.String())
	}

	updated, err := state.GetUserByUsername("mustchange2")
	if err != nil {
		t.Fatal(err)
	}
	if updated.RequirePasswordChange {
		t.Fatal("expected requirePasswordChange cleared")
	}
	loginReq := httptest.NewRequest(http.MethodPost, "/?username=mustchange2", nil)
	loginReq.Header.Set("X-Password", "newSecurePass")
	if _, err = auth.AuthenticatePassword(loginReq, true, false); err != nil {
		t.Fatalf("login with new password: %v", err)
	}
}
