package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func anonymousContext() *requestContext {
	return &requestContext{user: &users.User{Username: "anonymous"}}
}

func TestVerifyOTP_CachedSecretTakesPrecedenceWhenAuthenticatedSelf(t *testing.T) {
	setupTestEnv(t)

	oldSecret := "SOMEOLDSECRET234"
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

	reloaded, reloadErr := store.Users.Get(user.Username)
	if reloadErr != nil {
		t.Fatalf("failed to reload user: %v", reloadErr)
	}
	d := &requestContext{user: reloaded}

	rec := httptest.NewRecorder()
	if _, genErr := generateOTPHandler(rec, otpRequest(user.Username, "testPass", ""), d); genErr != nil {
		t.Fatalf("generation failed: %v", genErr)
	}
	var resp map[string]string
	if unmarshalErr := json.Unmarshal(rec.Body.Bytes(), &resp); unmarshalErr != nil {
		t.Fatalf("failed to decode response: %v", unmarshalErr)
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
	if _, verifyErr := verifyOTPHandler(httptest.NewRecorder(), otpRequest(user.Username, "testPass", oldCode), d); verifyErr == nil {
		t.Error("expected old secret to be rejected")
	}
	newCode, err := totp.GenerateCode(newSecret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate new code: %v", err)
	}
	if _, verifyErr := verifyOTPHandler(httptest.NewRecorder(), otpRequest(user.Username, "testPass", newCode), d); verifyErr != nil {
		t.Fatalf("expected new secret to be accepted: %v", verifyErr)
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

func TestRequireOtpEnrollmentAuthorized(t *testing.T) {
	withMFA := &users.User{
		Username:   "victim",
		TOTPSecret: "SOMEOLDSECRET234",
	}
	status, err := requireOtpEnrollmentAuthorized(withMFA, anonymousContext())
	if err == nil {
		t.Fatal("expected anonymous MFA reset to be rejected")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d err=%v", status, err)
	}

	noMFA := &users.User{Username: "new"}
	if status, err = requireOtpEnrollmentAuthorized(noMFA, anonymousContext()); err != nil {
		t.Fatalf("expected first-time enrollment to be allowed: status=%d err=%v", status, err)
	}

	self := &users.User{Username: "victim"}
	if status, err = requireOtpEnrollmentAuthorized(withMFA, &requestContext{user: self}); err != nil {
		t.Fatalf("expected self reset to be allowed: status=%d err=%v", status, err)
	}

	admin := &users.User{
		Username:    "admin",
		Permissions: users.Permissions{Admin: true},
	}
	if status, err = requireOtpEnrollmentAuthorized(withMFA, &requestContext{user: admin}); err != nil {
		t.Fatalf("expected admin reset to be allowed: status=%d err=%v", status, err)
	}

	other := &users.User{Username: "other"}
	status, err = requireOtpEnrollmentAuthorized(withMFA, &requestContext{user: other})
	if err == nil {
		t.Fatal("expected non-admin other user to be rejected")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d err=%v", status, err)
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
	if _, loginErr := loginHandler(httptest.NewRecorder(), otpRequest(user.Username, "", ""), d); loginErr != nil {
		t.Fatalf("expected login with TOTP to succed, got err=%v", loginErr)
	}
}
