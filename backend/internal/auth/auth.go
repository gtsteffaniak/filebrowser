package auth

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// Auther is the authentication interface.
type Auther interface {
	// Auth is called to authenticate a request.
	Auth(r *http.Request, userStore *users.Storage) (*users.User, error)
}

const FB_ISSUER = "FileBrowser Quantum" // don't change

// IsRevokedApiToken checks if a token is in the revoked list.
func IsRevokedApiToken(accessStore *access.Storage, token string) bool {
	if accessStore == nil {
		return false
	}
	return accessStore.IsTokenRevoked(token)
}

// RevokeApiToken adds a token to the revoked list.
func RevokeApiToken(accessStore *access.Storage, token string) error {
	if accessStore == nil {
		return fmt.Errorf("access storage not available")
	}
	return accessStore.RevokeToken(token)
}

func MakeSignedTokenAPI(user *users.User, name string, duration time.Duration, perms users.Permissions, minimal bool) (string, users.AuthToken, error) {
	if _, ok := user.Tokens[name]; ok {
		return "", users.AuthToken{}, fmt.Errorf("key already exists with same name %v ", name)
	}
	now := time.Now()
	expires := now.Add(duration)
	// Create minimal token with only JWT standard claims
	claim := users.AuthToken{
		MinimalAuthToken: users.MinimalAuthToken{
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(expires),
				Issuer:    FB_ISSUER,
			},
		},
	}
	if !minimal {
		claim.Permissions = users.SanitizeTokenPermissions(perms)
	}
	// Always sign minimal JWT claims only; identity is resolved via hashed_tokens lookup.
	signedClaims := jwt.Claims(claim.MinimalAuthToken)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, signedClaims)
	tokenString, err := token.SignedString([]byte(settings.Config.Auth.Key))
	if err != nil {
		return "", users.AuthToken{}, err
	}
	return tokenString, claim, nil

}
