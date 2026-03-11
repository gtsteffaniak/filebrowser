package auth

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// MethodNoAuth is used to identify no auth.
const MethodNoAuth = "noauth"

// NoAuth is no auth implementation of auther.
type NoAuth struct{}

// AuthenticateNoAuth authenticates as user 1 (admin) with no credentials required.
func AuthenticateNoAuth(r *http.Request, usr *users.Storage) (*users.User, error) {
	return usr.Get(uint(1))
}

// Auth uses authenticates user 1 (legacy method for compatibility).
func (a NoAuth) Auth(r *http.Request, usr *users.Storage) (*users.User, error) {
	return AuthenticateNoAuth(r, usr)
}
