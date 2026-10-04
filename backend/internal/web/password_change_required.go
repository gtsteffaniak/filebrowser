package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

type changeRequiredPasswordBody struct {
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

// changeRequiredPasswordHandler sets a new password when RequirePasswordChange is enabled, then issues a session token.
// @Summary Change required password at login
// @Description Verifies the current password and sets a new password when requirePasswordChange is set. Issues a JWT on success.
// @Tags Auth
// @Accept json
// @Produce json
// @Param username query string true "Username"
// @Param X-Password header string true "URL-encoded current password"
// @Param X-Secret header string false "TOTP code (if 2FA is enabled)"
// @Param body body changeRequiredPasswordBody true "New password and confirmation"
// @Success 200 {string} string "JWT token for authentication"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 401 {object} map[string]string "Unauthorized"
// @Failure 403 {object} map[string]string "Forbidden"
// @Router /api/auth/password/change-required [post]
func changeRequiredPasswordHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	user, err := auth.AuthenticatePassword(r, false, true)
	if err != nil {
		if err == errors.ErrNoTotpProvided {
			return http.StatusForbidden, err
		}
		return http.StatusUnauthorized, errors.ErrUnauthorized
	}
	if settings.Config.Auth.Methods.PasswordAuth.EnforcedOtp && user.TOTPSecret == "" {
		return http.StatusForbidden, errors.ErrNoTotpConfigured
	}
	if user.HasPasskeyMFA() && user.TOTPSecret == "" {
		return http.StatusForbidden, errors.ErrPasskeyMFARequired
	}
	if user.LoginMethod != users.LoginMethodPassword {
		return http.StatusBadRequest, fmt.Errorf("password change is only available for password login users")
	}
	if !user.RequirePasswordChange {
		return http.StatusForbidden, fmt.Errorf("password change is not required for this user")
	}

	var body changeRequiredPasswordBody
	if err = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid request body")
	}
	if body.Password == "" {
		return http.StatusBadRequest, errors.ErrEmptyPassword
	}
	if body.Password != body.PasswordConfirm {
		return http.StatusBadRequest, fmt.Errorf("passwords do not match")
	}
	if err = settings.ValidatePasswordPolicy(body.Password); err != nil {
		return http.StatusBadRequest, err
	}

	currentPassword := r.Header.Get("X-Password")
	currentPassword, decErr := url.QueryUnescape(currentPassword)
	if decErr != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid password encoding")
	}
	if err = utils.CheckPwd(currentPassword, user.Password); err != nil {
		return http.StatusUnauthorized, errors.ErrUnauthorized
	}
	if body.Password == currentPassword {
		return http.StatusBadRequest, fmt.Errorf("new password must differ from the current password")
	}

	patch := *user
	patch.RequirePasswordChange = false
	if err = state.UpdateUser(&patch, body.Password, "password", "requirePasswordChange"); err != nil {
		return http.StatusInternalServerError, err
	}

	updated, getErr := state.GetUserByUsername(user.Username)
	if getErr != nil {
		return http.StatusInternalServerError, getErr
	}
	d.User = &updated

	status, err := printToken(w, r, d.User, "")
	if err != nil || status != 0 {
		return status, err
	}
	activity.RecordLogin(r, d.User)
	return 0, nil
}
