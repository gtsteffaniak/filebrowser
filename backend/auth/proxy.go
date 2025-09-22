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

// Auth authenticates the user via an HTTP header.
func (a ProxyAuth) Auth(r *http.Request, usr *users.Storage) (*users.User, error) {
	username := r.Header.Get(a.Header)
	user, err := usr.Get(username)
	if err == errors.ErrNotExist {
		return nil, os.ErrPermission
	}

	return user, err
}
