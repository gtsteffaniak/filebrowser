package auth

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/ports"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// Service performs authentication using injected user reads. It never persists users.
type Service struct {
	users ports.UserReader
}

var defaultService *Service

// New constructs an auth service with the given user reader.
func New(users ports.UserReader) *Service {
	return &Service{users: users}
}

// SetDefault registers the process-wide auth service (called from app.WireServices).
func SetDefault(s *Service) {
	defaultService = s
}

// AuthenticatePassword authenticates the user via password in request headers.
func (s *Service) AuthenticatePassword(r *http.Request, disableOtp bool) (*users.User, error) {
	if s == nil || s.users == nil {
		return nil, fmt.Errorf("auth service not configured")
	}
	username := r.URL.Query().Get("username")
	password := r.Header.Get("X-Password")
	totpCode := r.Header.Get("X-Secret")
	password, err := url.QueryUnescape(password)
	if err != nil {
		return nil, fmt.Errorf("invalid password encoding: %v", err)
	}

	id, resErr := users.ResolveUsernameToID(username)
	var user users.User
	var getErr error
	if resErr != nil {
		getErr = resErr
	} else {
		user, getErr = s.users.GetUserByID(id)
	}
	var passwordHash string
	if getErr != nil {
		passwordHash = utils.InvalidPasswordHash
	} else {
		passwordHash = user.Password
	}
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
		err = VerifyTotpCode(&user, totpCode)
		if err != nil {
			return nil, err
		}
	}
	if user.LoginMethod != users.LoginMethodPassword {
		return nil, errors.ErrWrongLoginMethod
	}
	return &user, nil
}

// AuthenticatePassword authenticates via the default service.
func AuthenticatePassword(r *http.Request, disableOtp bool) (*users.User, error) {
	if defaultService != nil {
		return defaultService.AuthenticatePassword(r, disableOtp)
	}
	return nil, fmt.Errorf("auth service not configured")
}
