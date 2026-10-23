package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

func createTokenAuthUser(t *testing.T, username string, perms users.Permissions) *users.User {
	t.Helper()
	u := &users.User{FrontendUser: users.FrontendUser{Username: username}}
	if err := state.CreateUser(u, ""); err != nil {
		t.Fatal(err)
	}
	stored, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	stored.Permissions = perms
	if err = state.UpdateUser(&stored, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	return &reloaded
}

func cookieRequest(t *testing.T, handler http.HandlerFunc, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "filebrowser_quantum_jwt", Value: token})
	handler(rec, req)
	return rec
}
