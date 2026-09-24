package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func createTokenAuthUser(t *testing.T, username string, perms users.Permissions) *users.User {
	t.Helper()
	u := &users.User{FrontendUser: users.FrontendUser{Username: username}}
	if err := state.CreateUser(u, ""); err != nil {
		t.Fatal(err)
	}
	stored, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	stored.Permissions = perms
	if err = state.UpdateUser(&stored, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	return &reloaded
}

func cookieRequest(t *testing.T, handler http.HandlerFunc, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users", http.NoBody)
	req.AddCookie(&http.Cookie{Name: "filebrowser_quantum_jwt", Value: token})
	handler(rec, req)
	return rec
}

// A non-session bearer token whose hash has no stored permission metadata must be
// rejected rather than granted the owner's uncapped permissions.
func TestApiTokenWithoutMetadataRejected(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "metadata-less-admin", users.Permissions{Admin: true, Api: true})
	tokenString, _, err := auth.MakeSignedTokenAPI(user, "orphan-key", time.Hour, users.Permissions{}, true)
	if err != nil {
		t.Fatal(err)
	}
	// Register the hash mapping without any user.Tokens metadata.
	if err := state.AddApiToken(tokenString, user.ID); err != nil {
		t.Fatal(err)
	}

	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("metadata-less API token: got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// A named API token with stored caps must have those caps applied at auth time.
func TestApiTokenCapsAppliedFromStoredMetadata(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "capped-admin", users.Permissions{Admin: true, Api: true})

	// The token cap excludes admin, so the resolved user must lose admin.
	tokenString, tokenMeta, err := auth.MakeSignedTokenAPI(user, "capped-key", time.Hour, users.Permissions{Share: true}, false)
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

	rec := cookieRequest(t, withAdmin(mockHandler), tokenString)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("capped API token: got status %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// JwtAuth must reuse the session cookie it already issued instead of minting
// and registering a new session hash on every request.
func TestJwtAuthReusesSessionCookie(t *testing.T) {
	setupTestEnv(t)
	origAuthKey := settings.Config.Auth.Key
	origJwtAuth := settings.Config.Auth.Methods.JwtAuth
	origExp := settings.Config.Auth.TokenExpirationHours
	t.Cleanup(func() {
		settings.Config.Auth.Key = origAuthKey
		settings.Config.Auth.Methods.JwtAuth = origJwtAuth
		settings.Config.Auth.TokenExpirationHours = origExp
	})
	settings.Config.Auth.Key = "key"
	settings.Config.Auth.TokenExpirationHours = 2
	settings.Config.Auth.Methods.JwtAuth.Secret = "jwt-secret"
	settings.Config.Auth.Methods.JwtAuth.Algorithm = "HS256"
	settings.Config.Auth.Methods.JwtAuth.UserIdentifier = "sub"

	externalJWT, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "jwt-user",
	}).SignedString([]byte("jwt-secret"))
	if err != nil {
		t.Fatal(err)
	}

	call := func(cookieToken string) (*requestContext, *httptest.ResponseRecorder) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
		if cookieToken != "" {
			req.AddCookie(&http.Cookie{Name: "filebrowser_quantum_jwt", Value: cookieToken})
		}
		rec := httptest.NewRecorder()
		d := &requestContext{Ctx: req.Context()}
		status, err := getJwtUser(rec, req, d, mockHandler, externalJWT)
		if err != nil || status != http.StatusOK {
			t.Fatalf("getJwtUser: status=%d err=%v", status, err)
		}
		return d, rec
	}

	// First request without a cookie mints a session token and sets the cookie.
	d1, rec1 := call("")
	if d1.Token == "" {
		t.Fatal("expected a session token on first JwtAuth request")
	}
	if rec1.Header().Get("Set-Cookie") == "" {
		t.Fatal("expected session cookie on first JwtAuth request")
	}

	// Follow-up requests carrying the session cookie must reuse it instead of
	// minting and registering a new session.
	d2, _ := call(d1.Token)
	if d2.Token != d1.Token {
		t.Fatal("expected JwtAuth to reuse the existing session token")
	}
	d3, _ := call(d1.Token)
	if d3.Token != d1.Token {
		t.Fatal("expected repeated JwtAuth requests to reuse the session token")
	}
}

// A session cookie belonging to a different user must not be adopted by the
// JwtAuth user; a fresh session is minted instead.
func TestJwtAuthDoesNotReuseOtherUsersSessionCookie(t *testing.T) {
	setupTestEnv(t)
	origAuthKey := settings.Config.Auth.Key
	origJwtAuth := settings.Config.Auth.Methods.JwtAuth
	origExp := settings.Config.Auth.TokenExpirationHours
	t.Cleanup(func() {
		settings.Config.Auth.Key = origAuthKey
		settings.Config.Auth.Methods.JwtAuth = origJwtAuth
		settings.Config.Auth.TokenExpirationHours = origExp
	})
	settings.Config.Auth.Key = "key"
	settings.Config.Auth.TokenExpirationHours = 2
	settings.Config.Auth.Methods.JwtAuth.Secret = "jwt-secret"
	settings.Config.Auth.Methods.JwtAuth.Algorithm = "HS256"
	settings.Config.Auth.Methods.JwtAuth.UserIdentifier = "sub"

	other := createTokenAuthUser(t, "other-user", users.Permissions{})
	otherToken := testSessionToken(t, other, time.Hour)

	externalJWT, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "jwt-user",
	}).SignedString([]byte("jwt-secret"))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/resources", nil)
	req.AddCookie(&http.Cookie{Name: "filebrowser_quantum_jwt", Value: otherToken})
	rec := httptest.NewRecorder()
	d := &requestContext{Ctx: req.Context()}
	status, err := getJwtUser(rec, req, d, mockHandler, externalJWT)
	if err != nil || status != http.StatusOK {
		t.Fatalf("getJwtUser: status=%d err=%v", status, err)
	}
	if d.Token == "" || d.Token == otherToken {
		t.Fatal("expected a new session token, not the other user's cookie token")
	}
}

// WebDAV authenticates with Basic Auth where the password is the raw API JWT.
// A minimal named token with a hashed_tokens row and stored metadata must pass;
// the same JWT without a hash mapping must 401 (the 2.0.8-beta regression).
func TestApiTokenBasicAuthRoundTrip(t *testing.T) {
	setupTestEnv(t)
	originalAuthKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "key"
	t.Cleanup(func() { settings.Config.Auth.Key = originalAuthKey })

	user := createTokenAuthUser(t, "webdav-user", users.Permissions{Api: true})
	tokenString, tokenMeta, err := auth.MakeSignedTokenAPI(user, "obsidian-sync", time.Hour, users.Permissions{}, true)
	if err != nil {
		t.Fatal(err)
	}
	tokenMeta.Name = "obsidian-sync"
	tokenMeta.Token = tokenString
	if err = state.AddUserToken(user.Username, tokenMeta); err != nil {
		t.Fatal(err)
	}
	if err = state.AddApiToken(tokenString, user.ID); err != nil {
		t.Fatal(err)
	}

	basicRequest := func(token string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PROPFIND", "/dav/source1/", nil)
		req.SetBasicAuth("webdav-user", token)
		withBasicAuth(mockHandler)(rec, req)
		return rec
	}

	if rec := basicRequest(tokenString); rec.Code != http.StatusOK {
		t.Fatalf("backfilled API token over basic auth: got status %d, want %d", rec.Code, http.StatusOK)
	}

	// Same JWT shape with no hashed_tokens row must be rejected, never resolved
	// via legacy BelongsTo claims. A different duration guarantees a distinct
	// raw JWT (minimal claims carry only iss/iat/exp, so same-second mints
	// would otherwise be byte-identical).
	orphanString, _, err := auth.MakeSignedTokenAPI(user, "orphan-sync", 2*time.Hour, users.Permissions{}, true)
	if err != nil {
		t.Fatal(err)
	}
	rec := basicRequest(orphanString)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unmapped API token over basic auth: got status %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("expected WWW-Authenticate challenge on basic auth 401")
	}
}
