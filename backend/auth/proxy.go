package auth

import (
	"net/http"
	"os"

	"github.com/gtsteffaniak/filebrowser/settings"

	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/users"
)

// MethodProxyAuth is used to identify no auth.
const MethodProxyAuth = "proxy"

// ProxyAuth is a proxy implementation of an auther.
type ProxyAuth struct {
	Header string `json:"header"`
}

// Auth authenticates the user via an HTTP header.
func (a ProxyAuth) Auth(r *http.Request, usr users.Store) (*users.User, error) {
	username := r.Header.Get(a.Header)
	user, err := usr.Get(settings.GlobalConfiguration.Server.Root, username)
	if err == errors.ErrNotExist {
		return nil, os.ErrPermission
	}

	return user, err
}

// LoginPage tells that proxy auth doesn't require a login page.
func (a ProxyAuth) LoginPage() bool {
	return false
}
