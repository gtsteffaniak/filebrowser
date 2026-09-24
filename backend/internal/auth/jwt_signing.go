package auth

import (
	"fmt"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// JWTSigningKeyBytes returns the HMAC key or an error if it is unset (empty key must never verify).
func JWTSigningKeyBytes() ([]byte, error) {
	key := settings.Config.Auth.Key
	if len(key) == 0 {
		return nil, fmt.Errorf("auth signing key is not configured")
	}
	return []byte(key), nil
}

// JWTSigningKeyFunc is the keyFunc for FileBrowser-issued session and API JWTs.
func JWTSigningKeyFunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		return JWTSigningKeyBytes()
	}
}
