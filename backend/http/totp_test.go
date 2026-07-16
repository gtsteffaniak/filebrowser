package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/pquerna/otp/totp"
)

func otpRequest(username, password, code string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp?username="+username, http.NoBody)
	req.Header.Set("X-Password", password)
	if code != "" {
		req.Header.Set("X-Secret", code)
	}
	return req
}

func TestVerifyOTP_CachedSecretTakesPrecedence(t *testing.T) {
	setupTestEnv(t)

	oldSecret := "SOMEOLDSECRET123XD4"
	user := &users.User{
		ID:               1,
		Username:         "test",
		NonAdminEditable: users.NonAdminEditable{Password: "testPass"},
		TOTPSecret:       oldSecret,
		OtpEnabled:       true,
	}
	if err := store.Users.Save(user, true, true); err != nil {
		t.Fatalf("failed to save user: %v", err)
	}
	t.Cleanup(func() { auth.TotpCache.Delete(user.Username) })

	d := &requestContext{user: user}

	rec := httptest.NewRecorder()
	if _, err := generateOTPHandler(rec, otpRequest(user.Username, "testPass", ""), d); err != nil {
		t.Fatalf("genration failed: %v", err)
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	newSecret := strings.SplitN(resp["url"], "secret=", 2)[1]
	if newSecret == oldSecret {
		t.Fatal("expected a fresh secret")
	}
	oldCode, _ := totp.GenerateCode(oldSecret, time.Now())
	if _, err := verifyOTPHandler(httptest.NewRecorder(), otpRequest(user.Username, "testPass", oldCode), d); err == nil {
		t.Error("expected old secret to be rejected")
	}
	newCode, _ := totp.GenerateCode(newSecret, time.Now())
	if _, err := verifyOTPHandler(httptest.NewRecorder(), otpRequest(user.Username, "testPass", newCode), d); err != nil {
		t.Fatalf("expected new secret to be accepted: %v", err)
	}
	updated, err := store.Users.Get(user.Username)
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

func TestLogin_EnforcedOtp(t *testing.T) {
	setupTestEnv(t)
	config.Auth.Key = "test-key"
	config.Auth.TokenExpirationHours = 1
	config.Auth.Methods.PasswordAuth.EnforcedOtp = true

	user := &users.User{ID: 2, Username: "loginuser", LoginMethod: users.LoginMethodPassword}
	d := &requestContext{user: user}

	status, err := loginHandler(httptest.NewRecorder(), otpRequest(user.Username, "", ""), d)
	if status != http.StatusForbidden || err != errors.ErrNoTotpConfigured {
		t.Fatalf("expected login without TOTP to fail, got=%d err=%v", status, err)
	}

	user.TOTPSecret = "SOMESECRET123456"
	if _, err := loginHandler(httptest.NewRecorder(), otpRequest(user.Username, "", ""), d); err != nil {
		t.Fatalf("expected login with TOTP to succed, got err=%v", err)
	}
}
