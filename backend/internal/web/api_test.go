package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const webdavPasswordLimit = 256

func TestApiTokenCreationMode(t *testing.T) {
	t.Run("permissions minimal keyword", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/token?permissions=minimal", http.NoBody)
		minimal, perms := apiTokenCreationMode(req)
		if !minimal || perms != "" {
			t.Fatalf("got minimal=%v perms=%q", minimal, perms)
		}
	})
	t.Run("minimal true boolean", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/token?minimal=true", http.NoBody)
		minimal, _ := apiTokenCreationMode(req)
		if !minimal {
			t.Fatal("expected minimal")
		}
	})
	t.Run("minimal false with empty permissions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/token?minimal=false", http.NoBody)
		minimal, perms := apiTokenCreationMode(req)
		if minimal || perms != "" {
			t.Fatalf("got minimal=%v perms=%q", minimal, perms)
		}
	})
	t.Run("legacy omit both", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/token", http.NoBody)
		minimal, _ := apiTokenCreationMode(req)
		if !minimal {
			t.Fatal("expected legacy default minimal")
		}
	})
	t.Run("legacy permissions list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/token?permissions=api,admin", http.NoBody)
		minimal, perms := apiTokenCreationMode(req)
		if minimal || perms != "api,admin" {
			t.Fatalf("got minimal=%v perms=%q", minimal, perms)
		}
	})
}

func TestCreateApiTokenUncustomizedUnder256(t *testing.T) {
	setupTestEnv(t)
	setTestAuthKey(t)

	user := createApiTokenTestUser(t, "tokenuser", users.Permissions{Api: true})
	rec := invokeCreateApiToken(t, user, "/auth/token?name=webdav&days=365&permissions=minimal")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	token := decodeCreatedToken(t, rec.Body.Bytes())
	if len(token) >= webdavPasswordLimit {
		t.Fatalf("uncustomized token length %d must stay under %d", len(token), webdavPasswordLimit)
	}
	assertHandlerMinimalJWTClaims(t, token)
}

func TestCreateApiTokenLegacyOmitPermissionsStillMinimal(t *testing.T) {
	setupTestEnv(t)
	setTestAuthKey(t)

	user := createApiTokenTestUser(t, "tokenuser-legacy", users.Permissions{Api: true})
	rec := invokeCreateApiToken(t, user, "/auth/token?name=webdav&days=365")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertHandlerMinimalJWTClaims(t, decodeCreatedToken(t, rec.Body.Bytes()))
}

func TestCreateApiTokenCustomizedWithNoEffectiveCapsKeepsPermissionsClaim(t *testing.T) {
	setupTestEnv(t)
	setTestAuthKey(t)

	// User can create API tokens but lacks admin; explicit customized token keeps Permissions in JWT.
	user := createApiTokenTestUser(t, "tokenuser2", users.Permissions{Api: true})
	rec := invokeCreateApiToken(t, user, "/auth/token?name=custom&days=365&minimal=false&permissions=admin")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	token := decodeCreatedToken(t, rec.Body.Bytes())
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	perms, ok := claims["Permissions"].(map[string]interface{})
	if !ok {
		t.Fatalf("customized token should include Permissions claim, got %v", claims)
	}
	if perms["admin"] != false {
		t.Fatalf("expected admin false in customized token, got %v", perms["admin"])
	}
}

func TestCreateApiTokenCustomizedWithEffectiveCaps(t *testing.T) {
	setupTestEnv(t)
	setTestAuthKey(t)

	user := createApiTokenTestUser(t, "tokenuser3", users.Permissions{Api: true, Admin: true})
	rec := invokeCreateApiToken(t, user, "/auth/token?name=custom&days=30&permissions=admin,api")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	token := decodeCreatedToken(t, rec.Body.Bytes())
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, claims); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	perms, ok := claims["Permissions"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Permissions claim in customized token, got %v", claims)
	}
	if perms["admin"] != true || perms["api"] != true {
		t.Fatalf("expected admin and api true, got %v", perms)
	}
}

func setTestAuthKey(t *testing.T) {
	t.Helper()
	orig := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-for-length-check"
	t.Cleanup(func() { settings.Config.Auth.Key = orig })
}

func createApiTokenTestUser(t *testing.T, username string, perms users.Permissions) *users.User {
	t.Helper()
	user := &users.User{
		FrontendUser: users.FrontendUser{
			Username:    username,
			Permissions: perms,
		},
	}
	if err := state.CreateUser(user, "password"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	got, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	got.Permissions = perms
	if err = state.UpdateUser(&got, "", "permissions"); err != nil {
		t.Fatalf("update permissions: %v", err)
	}
	got, err = state.GetUserByUsername(username)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return &got
}

func invokeCreateApiToken(t *testing.T, user *users.User, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, http.NoBody)
	rec := httptest.NewRecorder()
	ctx := &Context{User: user, Ctx: req.Context()}
	status, err := createApiTokenHandler(rec, req, ctx)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, status, rec.Body.String())
	}
	return rec
}

func decodeCreatedToken(t *testing.T, body []byte) string {
	t.Helper()
	var resp HttpResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Token == "" {
		t.Fatalf("empty token in response: %s", body)
	}
	return resp.Token
}

func assertHandlerMinimalJWTClaims(t *testing.T, tokenString string) {
	t.Helper()
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(tokenString, claims); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	for _, key := range []string{"Permissions", "permissions", "belongsTo", "username", "name"} {
		if _, ok := claims[key]; ok {
			t.Fatalf("minimal token must not include %q claim; got %v", key, claims)
		}
	}
	if claims["iss"] != auth.FB_ISSUER {
		t.Fatalf("expected iss %q, got %v", auth.FB_ISSUER, claims["iss"])
	}
}
