package auth

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// MethodNoAuth is used to identify no auth.
const MethodNoAuth = "noauth"

// NoAuth is no auth implementation of auther.
type NoAuth struct{}

// AuthenticateNoAuth authenticates as the configured admin user with no credentials required.
func AuthenticateNoAuth(r *http.Request, user *users.Storage) (*users.User, error) {
	admin := settings.Config.Auth.AdminUsername
	if admin == "" {
		admin = "admin"
	}
	id, err := users.ResolveUsernameToID(admin)
	if err != nil {
		return nil, err
	}
	return user.Get(id)
}

// Auth uses authenticates as the configured admin user (legacy no-credentials mode).
func (a NoAuth) Auth(r *http.Request, user *users.Storage) (*users.User, error) {
	return AuthenticateNoAuth(r, user)
}
