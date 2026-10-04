package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestReplaceSessionTokenKeepsOldTokenWhenMintFails(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "rotation-user", users.Permissions{Api: true})
	oldToken := testSessionToken(t, user, time.Hour)

	invalidUser := *user
	invalidUser.ID = 0
	if _, err := replaceSessionToken(oldToken, &invalidUser); err == nil {
		t.Fatal("expected mint failure for user without an id")
	}
	if state.IsTokenRevoked(oldToken) {
		t.Fatal("old token must not be retired when replacement minting fails")
	}
	if _, _, ok := state.HashedTokenOwner(oldToken); !ok {
		t.Fatal("old token mapping must survive a failed replacement")
	}
}

func TestReplaceSessionTokenRetiresOldWithGrace(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "rotation-user-2", users.Permissions{Api: true})
	oldToken := testSessionToken(t, user, time.Hour)

	newToken, err := replaceSessionToken(oldToken, user)
	if err != nil {
		t.Fatal(err)
	}
	if newToken == oldToken {
		t.Fatal("expected a new session token")
	}
	if _, isSession, ok := state.HashedTokenOwner(newToken); !ok || !isSession {
		t.Fatalf("new token must be registered as a session: ok=%v isSession=%v", ok, isSession)
	}
	// The Playwright failures came from revoking the old token while in-flight
	// requests still carried it. It must stay valid during the grace window.
	if state.IsTokenRevoked(oldToken) {
		t.Fatal("old token must remain valid during the retirement grace window")
	}
	if _, _, ok := state.HashedTokenOwner(oldToken); !ok {
		t.Fatal("old token mapping must remain during the grace window")
	}
}

// Regression test for #3006: a request that already carried the session cookie
// must not re-emit Set-Cookie. Otherwise an in-flight response carrying a
// just-rotated-out token reverts the browser jar to a token that dies when its
// retirement grace expires.
func TestWithUserDoesNotEchoCookieToken(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "echo-user", users.Permissions{Api: true})
	token := testSessionToken(t, user, time.Hour)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})

	withUser(mockHandler)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if setCookie := recorder.Header().Values("Set-Cookie"); len(setCookie) != 0 {
		t.Fatalf("cookie-carried request must not re-emit Set-Cookie, got %v", setCookie)
	}
}

// Clients that authenticate without a session cookie (gvfs, Bearer, ?auth=)
// still get the token planted so subsequent requests stay authenticated.
func TestWithUserPlantsCookieForBearerClients(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "bearer-user", users.Permissions{Api: true})
	token := testSessionToken(t, user, time.Hour)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	withUser(mockHandler)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			session = c
		}
	}
	if session == nil {
		t.Fatal("cookie-less authenticated request must receive Set-Cookie")
	}
	if session.Value != token {
		t.Fatal("planted cookie must contain the presented token")
	}
}

// Restricted API tokens must not call renew: rotation registers a session hash
// and would grant the token owner's full permissions (GHSA-6gr6-5qpq-888p).
func TestRenewHandlerRejectsApiToken(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "renew-api-user", users.Permissions{Admin: true, Api: true})
	tokenString, tokenMeta, err := auth.MakeSignedTokenAPI(user, "capped-key", time.Hour, users.Permissions{Api: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	tokenMeta.Name = "capped-key"
	tokenMeta.Token = tokenString
	if err := state.AddUserToken(user.Username, tokenMeta); err != nil {
		t.Fatal(err)
	}
	if err := state.AddApiToken(tokenString, user.ID); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/renew", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+tokenString)

	status, err := renewHandler(recorder, req, &requestContext{User: user, Token: tokenString})
	if err == nil {
		t.Fatal("expected renew to fail for API token")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, status)
	}
	if _, isSession, ok := state.HashedTokenOwner(tokenString); !ok || isSession {
		t.Fatalf("API token must remain non-session: ok=%v isSession=%v", ok, isSession)
	}
}

// Renew while the presented token is still fresh returns it unchanged instead
// of rotating: routine page loads must not churn session-token state.
func TestRenewHandlerFreshTokenIsIdempotent(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "renew-fresh-user", users.Permissions{Api: true})
	token := testSessionToken(t, user, 2*time.Hour)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/renew", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})

	status, err := renewHandler(recorder, req, &requestContext{User: user, Token: token})
	if err != nil {
		t.Fatalf("renewHandler: %v", err)
	}
	if status != 0 && status != http.StatusOK {
		t.Fatalf("expected success status, got %d", status)
	}
	if got := recorder.Body.String(); got != token {
		t.Fatal("renew on a fresh token must return the same token")
	}
	if state.IsTokenRevoked(token) {
		t.Fatal("fresh token must not be retired by a no-op renew")
	}
	if setCookie := recorder.Header().Values("Set-Cookie"); len(setCookie) != 0 {
		t.Fatalf("idempotent renew with matching cookie must not re-emit Set-Cookie, got %v", setCookie)
	}
}

func TestRenewHandlerFreshTokenPlantsCookieWithoutJar(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "renew-fresh-no-cookie", users.Permissions{Api: true})
	token := testSessionToken(t, user, 2*time.Hour)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/renew", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)

	status, err := renewHandler(recorder, req, &requestContext{User: user, Token: token})
	if err != nil {
		t.Fatalf("renewHandler: %v", err)
	}
	if status != 0 && status != http.StatusOK {
		t.Fatalf("expected success status, got %d", status)
	}
	if len(recorder.Header().Values("Set-Cookie")) == 0 {
		t.Fatal("idempotent renew without session cookie must plant Set-Cookie")
	}
}

// Concurrent renew ordering: an in-flight idempotent renew must not revert a
// jar that a later rotation already upgraded.
func TestRenewHandlerIdempotentDoesNotRevertRotatedJar(t *testing.T) {
	setupTestEnv(t)
	user := createTokenAuthUser(t, "renew-order-user", users.Permissions{Api: true})
	oldToken := testSessionToken(t, user, 2*time.Hour)

	newToken, err := replaceSessionToken(oldToken, user)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/renew", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: oldToken})

	status, err := renewHandler(recorder, req, &requestContext{User: user, Token: oldToken})
	if err != nil {
		t.Fatalf("renewHandler: %v", err)
	}
	if status != 0 && status != http.StatusOK {
		t.Fatalf("expected success status, got %d", status)
	}
	if got := recorder.Body.String(); got != oldToken {
		t.Fatal("idempotent renew must still return the presented token")
	}
	if setCookie := recorder.Header().Values("Set-Cookie"); len(setCookie) != 0 {
		t.Fatalf("must not Set-Cookie old token after jar already rotated, got %v", setCookie)
	}
	if _, isSession, ok := state.HashedTokenOwner(newToken); !ok || !isSession {
		t.Fatalf("rotated token must remain registered: ok=%v isSession=%v", ok, isSession)
	}
}

func TestWithUserRefreshesCookieWhenBearerMismatchesJar(t *testing.T) {
	setupTestEnv(t)
	origExp := settings.Config.Auth.TokenExpirationHours
	t.Cleanup(func() { settings.Config.Auth.TokenExpirationHours = origExp })
	settings.Config.Auth.TokenExpirationHours = 2

	user := createTokenAuthUser(t, "bearer-mismatch-user", users.Permissions{Api: true})
	stale := testSessionToken(t, user, time.Hour)
	fresh, err := replaceSessionToken(stale, user)
	if err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+fresh)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: stale})

	withUser(mockHandler)(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	cookies := recorder.Result().Cookies()
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == sessionCookieName {
			session = c
		}
	}
	if session == nil {
		t.Fatalf("Bearer auth with stale cookie jar must refresh Set-Cookie, headers=%v", recorder.Header().Values("Set-Cookie"))
	}
	if session.Value != fresh {
		t.Fatal("planted cookie must match the Bearer token")
	}
}

// Inside the refresh window renew rotates: new token registered, old retired
// but still resolvable during the grace window.
func TestRenewHandlerNearExpiryRotates(t *testing.T) {
	setupTestEnv(t)
	origExp := settings.Config.Auth.TokenExpirationHours
	t.Cleanup(func() { settings.Config.Auth.TokenExpirationHours = origExp })
	settings.Config.Auth.TokenExpirationHours = 2

	user := createTokenAuthUser(t, "renew-rotate-user", users.Permissions{Api: true})
	token := testSessionToken(t, user, 10*time.Minute)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/renew", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})

	status, err := renewHandler(recorder, req, &requestContext{User: user, Token: token})
	if err != nil {
		t.Fatalf("renewHandler: %v", err)
	}
	if status != 0 && status != http.StatusOK {
		t.Fatalf("expected success status, got %d", status)
	}
	newToken := recorder.Body.String()
	if newToken == token {
		t.Fatal("renew inside the refresh window must rotate the token")
	}
	if _, isSession, ok := state.HashedTokenOwner(newToken); !ok || !isSession {
		t.Fatalf("rotated token must be registered as a session: ok=%v isSession=%v", ok, isSession)
	}
	if state.IsTokenRevoked(token) {
		t.Fatal("rotated-out token must remain valid during the grace window")
	}
}
