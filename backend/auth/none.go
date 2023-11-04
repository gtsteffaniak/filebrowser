package auth

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
)

// MethodNoAuth is used to identify no auth.
const MethodNoAuth = "noauth"

// NoAuth is no auth implementation of auther.
type NoAuth struct{}

// Auth uses authenticates user 1.
func (a NoAuth) Auth(r *http.Request, usr users.Store) (*users.User, error) {
	return usr.Get(settings.Config.Server.Root, uint(1))
}

// LoginPage tells that no auth doesn't require a login page.
func (a NoAuth) LoginPage() bool {
	return false
}
