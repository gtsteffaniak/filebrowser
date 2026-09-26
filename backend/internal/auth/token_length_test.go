package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const webdavPasswordLimit = 256

func TestMinimalSignedTokenLengthUnder256(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-for-length-check"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	user := &users.User{ID: 42, FrontendUser: users.FrontendUser{Username: "webdavuser"}}
	tokenString, _, err := MakeSignedTokenAPI(user, "webdav", time.Hour*24*365*10, users.Permissions{}, true)
	if err != nil {
		t.Fatalf("MakeSignedTokenAPI: %v", err)
	}
	if len(tokenString) >= webdavPasswordLimit {
		t.Fatalf("minimal token length %d must stay under %d for WebDAV clients", len(tokenString), webdavPasswordLimit)
	}
	assertMinimalJWTClaims(t, tokenString)
}

func TestCustomizedTokenMetadataRetainsPermissions(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-for-length-check"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	user := &users.User{ID: 42, FrontendUser: users.FrontendUser{Username: "webdavuser"}}
	_, claim, err := MakeSignedTokenAPI(user, "custom", time.Hour*24*365, users.Permissions{Api: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !claim.Permissions.Api {
		t.Fatalf("expected api permission in returned metadata: %+v", claim.Permissions)
	}
	tokenString, _, err := MakeSignedTokenAPI(user, "custom2", time.Hour*24*365, users.Permissions{Api: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	assertMinimalJWTClaims(t, tokenString)
}

func assertMinimalJWTClaims(t *testing.T, tokenString string) {
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
}
