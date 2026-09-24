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
