package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/pquerna/otp/totp"
)

func otpRequest(username, password, code, path string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path+"?username="+username, http.NoBody)
	req.Header.Set("X-Password", password)
	if code != "" {
		req.Header.Set("X-Secret", code)
	}
	return req
}

func TestVerifyOTP_CachedSecretTakesPrecedence(t *testing.T) {
	setupTestEnv(t)

	const oldSecret = "SOMEOLDSECRET234"
	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "test"},
	}
	if err := state.CreateUser(user, "testPass"); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	user.TOTPSecret = oldSecret
	user.OtpEnabled = true
	if err := state.UpdateUser(user, "", "TOTPSecret", "OtpEnabled"); err != nil {
		t.Fatalf("failed to set TOTP on user: %v", err)
	}
	t.Cleanup(func() { auth.TotpCache.Delete(user.Username) })

	d := &Context{User: user}

	rec := httptest.NewRecorder()
	if _, err := generateOTPHandler(rec, otpRequest(user.Username, "testPass", "", "/api/auth/otp/generate"), d); err != nil {
		t.Fatalf("generation failed: %v", err)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	u, err := url.Parse(resp["url"])
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	newSecret := u.Query().Get("secret")
	if newSecret == oldSecret {
		t.Fatal("expected a fresh secret")
	}
	oldCode, err := totp.GenerateCode(oldSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate old code: %v", err)
	}
	if _, verifyErr := verifyOTPHandler(httptest.NewRecorder(), otpRequest(user.Username, "testPass", oldCode, "/api/auth/otp/verify"), d); verifyErr == nil {
		t.Error("expected old secret to be rejected")
	}
	newCode, err := totp.GenerateCode(newSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate new code: %v", err)
	}
	if _, verifyErr := verifyOTPHandler(httptest.NewRecorder(), otpRequest(user.Username, "testPass", newCode, "/api/auth/otp/verify"), d); verifyErr != nil {
		t.Fatalf("expected new secret to be accepted: %v", verifyErr)
	}
	updated, err := state.GetUserByUsername(user.Username)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if updated.TOTPSecret != newSecret {
		t.Error("expected TOTPSecret to be updated to the new secret")
	}
	if _, found := auth.TotpCache.Get(user.Username); found {
		t.Error("expected cache to be cleared after verification")
	}
}

func TestLogin_RequiresOTPWhenTOTPSecretSet(t *testing.T) {
	setupTestEnv(t)

	origPasswordAuth := settings.Config.Auth.Methods.PasswordAuth.Enabled
	settings.Config.Auth.Methods.PasswordAuth.Enabled = true
	t.Cleanup(func() { settings.Config.Auth.Methods.PasswordAuth.Enabled = origPasswordAuth })

	const secret = "SOMEOLDSECRET234"
	user := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "test",
			LoginMethod: users.LoginMethodPassword,
		},
	}
	if err := state.CreateUser(user, "testPass"); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	user.TOTPSecret = secret
	user.OtpEnabled = true
	if err := state.UpdateUser(user, "", "TOTPSecret", "TOTPNonce", "OtpEnabled", "LoginMethod"); err != nil {
		t.Fatalf("failed to set TOTP on user: %v", err)
	}

	handler := wrapHandler(loginHelper(loginHandler))

	rec := httptest.NewRecorder()
	handler(rec, otpRequest(user.Username, "testPass", "", "/api/auth/login"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without OTP, got %d body=%s", rec.Code, rec.Body.String())
	}
	var errResp HttpResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Message != "OTP code is required for user" {
		t.Fatalf("expected OTP required message, got %q", errResp.Message)
	}

	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate TOTP code: %v", err)
	}
	rec2 := httptest.NewRecorder()
	handler(rec2, otpRequest(user.Username, "testPass", code, "/api/auth/login"))
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid OTP, got %d body=%s", rec2.Code, rec2.Body.String())
	}
}
