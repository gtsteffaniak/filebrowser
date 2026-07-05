package auth

import (
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// JSONAuth is a json implementation of an Auther.
type JSONAuth struct {
	ReCaptcha  bool `json:"recaptcha" yaml:"recaptcha"`
	DisableOtp bool `json:"disableOtp" yaml:"disableOtp"`
}

// Auth authenticates the user via a json in content body (legacy method for compatibility).
func (auther JSONAuth) Auth(r *http.Request, _ *users.Storage) (*users.User, error) {
	return AuthenticatePassword(r, auther.ReCaptcha)
}
