package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// wopiTokenAudience keeps WOPI access tokens and session tokens apart: a WOPI
// token is signed with its own key and carries this audience, so neither can
// be replayed where the other is expected.
const wopiTokenAudience = "wopi"

// wopiClaims is what an access token binds: one file, one user, one mode.
// The token is the only credential the editor presents, so everything the
// WOPI endpoints need to re-check permissions travels in it.
type wopiClaims struct {
	FileID string `json:"fid"`
	Source string `json:"src"`
	Path   string `json:"path"` // relative to the user's scope in Source
	UserID uint64 `json:"uid"`
	Write  bool   `json:"w"`
	Origin string `json:"org"` // browser origin allowed to postMessage with the editor
	jwt.RegisteredClaims
}

// wopiFileID derives the WOPI file id from where the file lives. It is opaque
// to the editor and stable for the life of the file, so concurrent sessions on
// one document share it and the editor joins them in one co-editing session.
func wopiFileID(source, realPath string) string {
	sum := sha256.Sum256([]byte(source + "\x00" + realPath))
	return hex.EncodeToString(sum[:])
}

// wopiSigningKey returns integrations.wopi.secret, or a key derived from the
// server's auth key so tokens survive a restart without any extra setting.
func wopiSigningKey() ([]byte, error) {
	if secret := settings.Config.Integrations.Wopi.Secret; secret != "" {
		return []byte(secret), nil
	}
	authKey, err := auth.JWTSigningKeyBytes()
	if err != nil {
		return nil, err
	}
	mac := hmac.New(sha256.New, authKey)
	mac.Write([]byte("filebrowser-wopi-access-token"))
	return mac.Sum(nil), nil
}

func wopiTokenTTL() time.Duration {
	hours := settings.Config.Integrations.Wopi.TokenExpirationHours
	if hours <= 0 {
		hours = 10
	}
	return time.Duration(hours) * time.Hour
}

// mintWopiToken signs an access token and returns it with its expiry.
func mintWopiToken(c wopiClaims, now time.Time) (string, time.Time, error) {
	key, err := wopiSigningKey()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := now.Add(wopiTokenTTL())
	c.RegisteredClaims = jwt.RegisteredClaims{
		Audience:  jwt.ClaimStrings{wopiTokenAudience},
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expires),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(key)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expires, nil
}

// parseWopiToken verifies signature, algorithm, audience and expiry, and that
// the token was minted for fileID.
func parseWopiToken(raw, fileID string) (*wopiClaims, error) {
	if raw == "" {
		return nil, errors.New("missing access_token")
	}
	key, err := wopiSigningKey()
	if err != nil {
		return nil, err
	}
	var c wopiClaims
	_, err = jwt.ParseWithClaims(raw, &c, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid access_token: %w", err)
	}
	if !c.VerifyAudience(wopiTokenAudience, true) {
		return nil, errors.New("access_token is not a wopi token")
	}
	if c.ExpiresAt == nil {
		return nil, errors.New("access_token has no expiry")
	}
	if !hmac.Equal([]byte(c.FileID), []byte(fileID)) {
		return nil, errors.New("access_token was issued for another file")
	}
	return &c, nil
}
