package auth

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// MethodNoAuth is used to identify no auth.
const MethodNoAuth = "noauth"

// NoAuth is no auth implementation of auther.
type NoAuth struct{}

// Auth uses authenticates user 1.
func (a NoAuth) Auth(r *http.Request, usr *users.Storage) (*users.User, error) {
	return usr.Get(uint(1))
}
