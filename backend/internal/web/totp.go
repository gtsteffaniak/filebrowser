package web

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// generateOTPHandler handles the generation of a new TOTP secret and QR code.
// @Summary Generate OTP
// @Description Generates a new TOTP secret and QR code for the authenticated user. The password must be URL-encoded and sent in the X-Password header to support special characters.
// @Tags Auth
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param username query string true "Username"
// @Param X-Password header string true "URL-encoded password"
// @Success 200 {object} map[string]string "OTP secret generated successfully."
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/auth/otp/generate [post]
func generateOTPHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	targetUsername := r.URL.Query().Get("username")
	user, getErr := state.GetUserByUsername(targetUsername)
	if getErr != nil {
		return http.StatusNotFound, fmt.Errorf("user not found: %w", getErr)
	}
	if status, err := requireOtpEnrollmentAuthorized(&user, d); err != nil {
		return status, err
	}
	if err := checkOtpActorPassword(r, d, user.Username); err != nil {
		return http.StatusUnauthorized, err
	}
	url, err := auth.GenerateOtpForUser(&user)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error generating OTP secret: %w", err)
	}
	response := map[string]string{
		"message": "OTP secret generated successfully.",
		"url":     url, // The otpauth:// URL for QR code generation
	}
	return RenderJSON(w, r, response)
}

// verifyOTPHandler handles the verification of a TOTP code.
// @Summary Verify OTP
// @Description Verifies the provided TOTP code for the authenticated user. The password must be URL-encoded and sent in the X-Password header to support special characters.
// @Tags Auth
// @Accept json
// @Produce json
// @Security ApiKeyAuth
// @Param username query string true "Username"
// @Param X-Password header string true "URL-encoded password"
// @Param X-Secret header string true "TOTP code to verify"
// @Success 200 {object} HttpResponse "OTP token is valid."
// @Failure 401 {object} map[string]string "Unauthorized - invalid TOTP token"
// @Router /api/auth/otp/verify [post]
func verifyOTPHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	code := r.Header.Get("X-Secret")
	if code == "" {
		return http.StatusUnauthorized, fmt.Errorf("code is required")
	}
	targetUsername := r.URL.Query().Get("username")
	user, getErr := state.GetUserByUsername(targetUsername)
	if getErr != nil {
		return http.StatusNotFound, fmt.Errorf("user not found: %w", getErr)
	}
	if status, err := requireOtpEnrollmentAuthorized(&user, d); err != nil {
		return status, err
	}
	if err := checkOtpActorPassword(r, d, user.Username); err != nil {
		return http.StatusUnauthorized, err
	}
	if err := auth.VerifyTotpCode(&user, code); err != nil {
		return http.StatusUnauthorized, fmt.Errorf("invalid OTP token")
	}
	if err := state.UpdateUser(&user, "", "TOTPSecret", "TOTPNonce", "OtpEnabled"); err != nil {
		return http.StatusInternalServerError, err
	}
	response := HttpResponse{
		Status:  http.StatusOK,
		Message: "OTP token is valid.",
	}
	// On success, return a simple confirmation.
	return RenderJSON(w, r, response)
}

// userConfiguredMFA reports whether the account already has a second factor that must not be reset anonymously.
func userConfiguredMFA(u *users.User) bool {
	if u == nil {
		return false
	}
	if u.HasPasskeyMFA() {
		return true
	}
	return u.TOTPSecret != ""
}

// requireOtpEnrollmentAuthorized allows password-only OTP enrollment only for first-time setup (no MFA).
// Replacing or adding TOTP when MFA exists requires an authenticated self or admin session.
func requireOtpEnrollmentAuthorized(target *users.User, d *Context) (int, error) {
	if !userConfiguredMFA(target) {
		return 0, nil
	}
	if d.User == nil || d.User.Username == "" || d.User.Username == "anonymous" {
		return http.StatusForbidden, fmt.Errorf("authentication required to reset two-factor authentication")
	}
	if d.User.Permissions.Admin || d.User.Username == target.Username {
		return 0, nil
	}
	return http.StatusForbidden, fmt.Errorf("not authorized to reset two-factor authentication for this user")
}

// checkOtpActorPassword verifies X-Password for the account that must prove knowledge of the password.
// For admin sessions resetting another user, the admin's password is checked.
func checkOtpActorPassword(r *http.Request, d *Context, targetUsername string) error {
	providedPassword := r.Header.Get("X-Password")
	actorUsername := targetUsername
	if d.User != nil && d.User.Permissions.Admin && d.User.Username != targetUsername {
		actorUsername = d.User.Username
	}
	providedPassword, err := url.QueryUnescape(providedPassword)
	if err != nil {
		return fmt.Errorf("invalid password encoding: %v", err)
	}
	if providedPassword == "" {
		return fmt.Errorf("password is required")
	}
	user, getErr := state.GetUserByUsername(actorUsername)
	var passwordHash string
	if getErr != nil {
		passwordHash = utils.InvalidPasswordHash
	} else {
		passwordHash = user.Password
	}
	err = utils.CheckPwd(providedPassword, passwordHash)
	if err != nil {
		return fmt.Errorf("invalid password or user not found")
	}
	return nil
}
