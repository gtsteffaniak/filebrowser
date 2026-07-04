package auth

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// JSONAuth is a json implementation of an Auther.
type JSONAuth struct {
	ReCaptcha  bool `json:"recaptcha" yaml:"recaptcha"`
	DisableOtp bool `json:"disableOtp" yaml:"disableOtp"`
}

// Auth authenticates the user via a json in content body (legacy method for compatibility).
func (auther JSONAuth) Auth(r *http.Request, userStore *users.Storage) (*users.User, error) {
	return AuthenticatePassword(r, userStore, auther.ReCaptcha)
}

// AuthenticatePassword authenticates the user via password in request headers.
func AuthenticatePassword(r *http.Request, userStore *users.Storage, disableOtp bool) (*users.User, error) {
	username := r.URL.Query().Get("username")
	password := r.Header.Get("X-Password")
	totpCode := r.Header.Get("X-Secret")
	// URL-decode password to support special characters in headers
	password, err := url.QueryUnescape(password)
	if err != nil {
		return nil, fmt.Errorf("invalid password encoding: %v", err)
	}

	id, resErr := users.ResolveUsernameToID(username)
	var user *users.User
	var getErr error
	if resErr != nil {
		getErr = resErr
	} else {
		user, getErr = userStore.Get(id)
	}
	var passwordHash string
	if getErr != nil {
		passwordHash = utils.InvalidPasswordHash
	} else {
		passwordHash = user.Password
	}
	// always run checkPwd to prevent timing attacks
	err = utils.CheckPwd(password, passwordHash)
	if err != nil {
		return nil, err
	}
	if getErr != nil {
		return nil, fmt.Errorf("unable to get user from store: %v", err)
	}
	if user.TOTPSecret != "" && !disableOtp {
		if totpCode == "" {
			return nil, errors.ErrNoTotpProvided
		}
		err = VerifyTotpCode(user, totpCode, userStore)
		if err != nil {
			return nil, err
		}
	}

	if user.LoginMethod != users.LoginMethodPassword {
		return nil, errors.ErrWrongLoginMethod
	}

	return user, nil
}
