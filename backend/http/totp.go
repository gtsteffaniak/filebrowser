package http

import (
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
)

// generateOTPHandler handles the generation of a new TOTP secret and QR code.
func generateOTPHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	fmt.Println("Generating OTP for user:", d.user.Username)
	url, err := auth.GenerateOtpForUser(d.user, store.Users)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error generating OTP secret: %w", err)
	}
	response := map[string]string{
		"message": "OTP secret generated successfully.",
		"url":     url, // The otpauth:// URL for QR code generation
	}
	return renderJSON(w, r, response)
}

// verifyOTPAHandler handles the verification of a TOTP code.
func verifyOTPHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	code := r.URL.Query().Get("code")
	if code == "" {
		return http.StatusUnauthorized, fmt.Errorf("code is required")
	}
	err := auth.VerifyTotpCode(d.user, code, store.Users)
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
