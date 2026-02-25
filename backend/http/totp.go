package http

import (
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/go-logger/logger"
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
	username := r.URL.Query().Get("username")
	if username == "" {
		username = d.user.Username
	}
	if username != d.user.Username && !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("you are not authorized to generate OTP for this user")
	}
	targetUser, err := store.Users.Get(username)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("user not found: %w", err)
	}
	url, err := auth.GenerateOtpForUser(targetUser, store.Users)
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

	targetUser := d.user
	username := r.URL.Query().Get("username")

	// If admin is verifying for another user
	if username != "" && username != d.user.Username && d.user.Permissions.Admin {
		var err error
		targetUser, err = store.Users.Get(username)
		if err != nil {
			return http.StatusNotFound, fmt.Errorf("user not found: %w", err)
		}
		logger.Debugf("Admin %s verifying OTP for user: %s", d.user.Username, targetUser.Username)
	}

	err := auth.VerifyTotpCode(targetUser, code, store.Users)
	if err != nil {
		return http.StatusUnauthorized, fmt.Errorf("invalid OTP token")
	}
	response := HttpResponse{
		Status:  http.StatusOK,
		Message: "OTP token is valid.",
	}
	// On success, return a simple confirmation.
	return renderJSON(w, r, response)
}
