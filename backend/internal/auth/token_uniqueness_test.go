package auth

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestSignedTokensAreUniqueAtSameTime(t *testing.T) {
	origKey := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-for-token-uniqueness"
	t.Cleanup(func() { settings.Config.Auth.Key = origKey })

	// Freeze the clock so every token has identical issue and expiry times.
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		tokens := make(map[string]users.AuthToken)
		for _, userID := range []uint64{1, 2} {
			user := &users.User{ID: userID}
			for _, minimal := range []bool{true, false} {
				for range 2 {
					tokenString, claim, err := MakeSignedTokenAPI(user, "test-token", time.Hour, users.Permissions{Api: true}, minimal)
					if err != nil {
						t.Fatal(err)
					}
					if _, exists := tokens[tokenString]; exists {
						t.Fatalf("token collision for user %d with minimal=%v", userID, minimal)
					}
					tokens[tokenString] = claim
				}
			}
		}
		ids := make(map[string]bool)
		for tokenString, claim := range tokens {
			var parsed users.MinimalAuthToken
			if _, err := jwt.ParseWithClaims(tokenString, &parsed, JWTSigningKeyFunc()); err != nil {
				t.Fatalf("parse signed token: %v", err)
			}
			if parsed.ID == "" || ids[parsed.ID] {
				t.Fatal("each signed token must have a unique nonempty id")
			}
			ids[parsed.ID] = true
			if parsed.ID != claim.ID {
				t.Fatal("signed token id must match returned metadata")
			}
			if parsed.IssuedAt == nil || !parsed.IssuedAt.Equal(now) || parsed.ExpiresAt == nil || !parsed.ExpiresAt.Equal(now.Add(time.Hour)) {
				t.Fatal("token uniqueness must not depend on changing timestamps")
			}
			assertMinimalJWTClaims(t, tokenString)
		}
	})
}
