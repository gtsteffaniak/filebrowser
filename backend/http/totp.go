package http

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
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
func generateOTPHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	targetUsername := r.URL.Query().Get("username")
	user, getErr := store.Users.Get(targetUsername)
	if getErr != nil {
		return http.StatusNotFound, fmt.Errorf("user not found: %w", getErr)
	}
	if status, err := requireOtpEnrollmentAuthorized(user, d); err != nil {
		return status, err
	}
	if err := checkOtpActorPassword(r, d, user.Username); err != nil {
		return http.StatusUnauthorized, err
	}
	url, err := auth.GenerateOtpForUser(user, store.Users)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error generating OTP secret: %w", err)
	}
	response := map[string]string{
		"message": "OTP secret generated successfully.",
		"url":     url, // The otpauth:// URL for QR code generation
	}
	return renderJSON(w, r, response)
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
func verifyOTPHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	code := r.Header.Get("X-Secret")
	if code == "" {
		return http.StatusUnauthorized, fmt.Errorf("code is required")
	}
	targetUsername := r.URL.Query().Get("username")
	user, getErr := store.Users.Get(targetUsername)
	if getErr != nil {
		return http.StatusNotFound, fmt.Errorf("user not found: %w", getErr)
	}
	if status, err := requireOtpEnrollmentAuthorized(user, d); err != nil {
		return status, err
	}
	if err := checkOtpActorPassword(r, d, user.Username); err != nil {
		return http.StatusUnauthorized, err
	}
	if err := auth.VerifyTotpCode(user, code, store.Users); err != nil {
		return http.StatusUnauthorized, fmt.Errorf("invalid OTP token")
	}
	response := HttpResponse{
		Status:  http.StatusOK,
		Message: "OTP token is valid.",
	}
	// On success, return a simple confirmation.
	return renderJSON(w, r, response)
}

func userConfiguredMFA(u *users.User) bool {
	if u == nil {
		return false
	}
	if u.HasPasskeyMFA() {
		return true
	}
	return u.TOTPSecret != ""
}

func requireOtpEnrollmentAuthorized(target *users.User, d *requestContext) (int, error) {
	if !userConfiguredMFA(target) {
		return 0, nil
	}
	if d.user == nil || d.user.Username == "" || d.user.Username == "anonymous" {
		return http.StatusForbidden, fmt.Errorf("authentication required to reset two-factor authentication")
	}
	if d.user.Permissions.Admin || d.user.Username == target.Username {
		return 0, nil
	}
	return http.StatusForbidden, fmt.Errorf("not authorized to reset two-factor authentication for this user")
}

func checkOtpActorPassword(r *http.Request, d *requestContext, targetUsername string) error {
	providedPassword := r.Header.Get("X-Password")
	actorUsername := targetUsername
	if d.user != nil && d.user.Permissions.Admin && d.user.Username != targetUsername {
		actorUsername = d.user.Username
	}
	providedPassword, err := url.QueryUnescape(providedPassword)
	if err != nil {
		return fmt.Errorf("invalid password encoding: %v", err)
	}
	if providedPassword == "" {
		return fmt.Errorf("password is required")
	}
	user, getErr := store.Users.Get(actorUsername)
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
