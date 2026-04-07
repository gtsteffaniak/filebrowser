package auth

import (
	"net/http"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// MethodProxyAuth is used to identify no auth.
const MethodProxyAuth = "proxy"

// ProxyAuth is a proxy implementation of an auther.
type ProxyAuth struct {
	Header string `json:"header"`
}

// AuthenticateProxy authenticates the user via an HTTP header.
func AuthenticateProxy(r *http.Request, usr *users.Storage, headerName string) (*users.User, error) {
	username := r.Header.Get(headerName)
	id, err := users.ResolveUsernameToID(username)
	if err == errors.ErrNotExist {
		return nil, os.ErrPermission
	}
	if err != nil {
		return nil, err
	}
	user, err := usr.Get(id)
	if err == errors.ErrNotExist {
		return nil, os.ErrPermission
	}

	return user, err
}

// Auth authenticates the user via an HTTP header (legacy method for compatibility).
func (a ProxyAuth) Auth(r *http.Request, usr *users.Storage) (*users.User, error) {
	return AuthenticateProxy(r, usr, a.Header)
}
