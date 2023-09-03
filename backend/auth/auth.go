package auth

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/users"
)

// Auther is the authentication interface.
type Auther interface {
	// Auth is called to authenticate a request.
	Auth(r *http.Request, usr users.Store) (*users.User, error)
	// LoginPage indicates if this auther needs a login page.
	LoginPage() bool
}
