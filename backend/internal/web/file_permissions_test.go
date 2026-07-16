package web

import (
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func signAuthToken(t *testing.T, perms users.Permissions) string {
	t.Helper()
	origKey := settings.Config.Auth.Key
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	key := []byte("test-auth-key")
	settings.Config.Auth.Key = string(key)

	claims := users.AuthToken{
		MinimalAuthToken: users.MinimalAuthToken{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		},
		Permissions: perms,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	return signed
}

func testUserWithSourcePerms(sourcePath string, perms users.SourceFilePermissions) *users.User {
	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "alice"},
		BackendScopes: []users.BackendScope{
			{Path: sourcePath, Scope: "/", Permissions: perms},
		},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			sourcePath: perms,
		},
		Version: users.SourcePermissionsMigrationVersion,
	}
	return user
}

func TestAPITokenSourceFilePerms(t *testing.T) {
	tests := []struct {
		name      string
		tokenCaps users.Permissions
		wantOK    bool
		want      users.SourceFilePermissions
	}{
		{
			name:   "legacy token without file caps",
			wantOK: false,
		},
		{
			name:      "some caps set",
			tokenCaps: users.Permissions{View: true, Download: true},
			wantOK:    true,
			want:      users.SourceFilePermissions{View: true, Download: true},
		},
		{
			name:      "all caps explicitly false",
			tokenCaps: users.Permissions{View: false, Download: false, Modify: false, Delete: false, Create: false},
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &requestContext{Token: signAuthToken(t, tt.tokenCaps)}
			got, ok := apiTokenSourceFilePerms(d)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("perms = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEffectiveFilePermsIntersectsTokenCaps(t *testing.T) {
	initStreamGrantTestSources(t)

	sourcePath := "/default"
	userPerms := users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
	d := &requestContext{
		User:  testUserWithSourcePerms(sourcePath, userPerms),
		Token: signAuthToken(t, users.Permissions{View: true, Download: false}),
	}

	got, err := effectiveFilePerms(d, "default")
	if err != nil {
		t.Fatalf("effectiveFilePerms: %v", err)
	}
	want := users.SourceFilePermissions{View: true}
	if got != want {
		t.Fatalf("effectiveFilePerms = %+v, want %+v", got, want)
	}
}
