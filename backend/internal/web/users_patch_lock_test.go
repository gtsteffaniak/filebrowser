package web

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

func TestUserPatchLockPassword(t *testing.T) {
	setupTestEnv(t)

	admin := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "admin",
			LoginMethod: users.LoginMethodOidc,
			Permissions: users.Permissions{Admin: true},
		},
	}
	if err := state.CreateUser(admin, ""); err != nil {
		t.Fatal(err)
	}
	admin.Permissions = users.Permissions{Admin: true}
	if err := state.UpdateUser(admin, "", "permissions"); err != nil {
		t.Fatal(err)
	}

	locked := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "locked",
			LoginMethod: users.LoginMethodPassword,
		},
	}
	if err := state.CreateUser(locked, "oldpassword"); err != nil {
		t.Fatal(err)
	}
	locked.LockPassword = true
	if err := state.UpdateUser(locked, "", "lockPassword"); err != nil {
		t.Fatal(err)
	}
	locked, err := getUser(t, "locked")
	if err != nil {
		t.Fatal(err)
	}
	if !locked.LockPassword {
		t.Fatal("lockPassword was not stored")
	}
	oldHash := locked.Password

	patch := func(actor *users.User) int {
		body := []byte(`{"which":["password"],"data":{"password":"newpassword"}}`)
		req := httptest.NewRequest(http.MethodPatch, "/api/users?username=locked", bytes.NewReader(body))
		req.Header.Set("X-Password", "oldpassword")
		status, _ := userPatchHandler(httptest.NewRecorder(), req, &Context{User: actor})
		return status
	}

	if status := patch(locked); status != http.StatusForbidden {
		t.Fatalf("self password change with lockPassword: status=%d, want %d", status, http.StatusForbidden)
	}
	after, err := getUser(t, "locked")
	if err != nil {
		t.Fatal(err)
	}
	if after.Password != oldHash {
		t.Fatal("password changed despite lockPassword")
	}

	if status := patch(admin); status != http.StatusNoContent {
		t.Fatalf("admin password reset of locked user: status=%d, want %d", status, http.StatusNoContent)
	}
}

func getUser(t *testing.T, username string) (*users.User, error) {
	t.Helper()
	u, err := state.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
