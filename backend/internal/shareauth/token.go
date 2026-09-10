package shareauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const (
	ScopeDownload = "download"
	ScopeUISession  = "ui"
	maxAccessTTL    = 24 * time.Hour
)

type accessClaims struct {
	Hash  string `json:"h"`
	Exp   int64  `json:"exp"`
	Scope string `json:"s"`
	ID    string `json:"id,omitempty"`
}

func parseDuration(duration int, unit string) (time.Duration, error) {
	if duration <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	switch unit {
	case "minutes", "minute", "":
		return time.Duration(duration) * time.Minute, nil
	case "hours", "hour":
		return time.Duration(duration) * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid unit: %s", unit)
	}
}

// ParseAccessDuration validates and returns TTL capped at 24 hours.
func ParseAccessDuration(duration int, unit string) (time.Duration, error) {
	ttl, err := parseDuration(duration, unit)
	if err != nil {
		return 0, err
	}
	if ttl > maxAccessTTL {
		return 0, fmt.Errorf("duration exceeds maximum of 24 hours")
	}
	return ttl, nil
}

// MintDownloadAccessToken creates a signed, download-scoped ephemeral token for a share.
func MintDownloadAccessToken(shareHash string, ttl time.Duration, maxUses int) (token string, expiresAt int64, err error) {
	if shareHash == "" {
		return "", 0, fmt.Errorf("share hash is required")
	}
	if ttl <= 0 || ttl > maxAccessTTL {
		return "", 0, fmt.Errorf("invalid token duration")
	}
	if maxUses < 0 {
		return "", 0, fmt.Errorf("invalid max uses")
	}

	expiresAt = time.Now().Add(ttl).Unix()
	claims := accessClaims{
		Hash:  shareHash,
		Exp:   expiresAt,
		Scope: ScopeDownload,
	}

	remainingUses := -1
	if maxUses > 0 {
		idBytes := make([]byte, 12)
		if _, err = rand.Read(idBytes); err != nil {
			return "", 0, err
		}
		claims.ID = base64.RawURLEncoding.EncodeToString(idBytes)
		remainingUses = maxUses
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", 0, err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, []byte(settings.Config.Auth.Key))
	mac.Write([]byte(payloadB64))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	token = payloadB64 + "." + signature

	if claims.ID != "" {
		utils.ShareAccessGrantsCache.Set(claims.ID, utils.ShareAccessGrant{RemainingUses: remainingUses})
	}

	return token, expiresAt, nil
}

// MintShareUISessionToken creates a signed cookie-scoped token for UI share access after password entry.
func MintShareUISessionToken(shareHash string, ttl time.Duration) (token string, expiresAt int64, err error) {
	if shareHash == "" {
		return "", 0, fmt.Errorf("share hash is required")
	}
	if ttl <= 0 || ttl > maxAccessTTL {
		return "", 0, fmt.Errorf("invalid token duration")
	}

	expiresAt = time.Now().Add(ttl).Unix()
	claims := accessClaims{
		Hash:  shareHash,
		Exp:   expiresAt,
		Scope: ScopeUISession,
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", 0, err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)

	mac := hmac.New(sha256.New, []byte(settings.Config.Auth.Key))
	mac.Write([]byte(payloadB64))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	token = payloadB64 + "." + signature
	return token, expiresAt, nil
}

// ValidateShareUISessionToken checks a UI session token for the given share hash.
func ValidateShareUISessionToken(tokenParam, shareHash string) bool {
	claims, ok := verifySignedToken(tokenParam)
	if !ok || claims == nil {
		return false
	}
	if claims.Scope != ScopeUISession || claims.Hash != shareHash {
		return false
	}
	return time.Now().Unix() <= claims.Exp
}

func verifySignedToken(tokenParam string) (*accessClaims, bool) {
	parts := strings.Split(tokenParam, ".")
	if len(parts) != 2 {
		return nil, false
	}
	payloadB64, signature := parts[0], parts[1]

	mac := hmac.New(sha256.New, []byte(settings.Config.Auth.Key))
	mac.Write([]byte(payloadB64))
	expectedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return nil, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, false
	}

	var claims accessClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, false
	}
	return &claims, true
}

// ValidateDownloadAccessToken checks a download-scoped token for the given share hash.
func ValidateDownloadAccessToken(tokenParam, shareHash string) bool {
	claims, ok := verifySignedToken(tokenParam)
	if !ok || claims == nil {
		return false
	}
	if claims.Scope != ScopeDownload || claims.Hash != shareHash {
		return false
	}
	if time.Now().Unix() > claims.Exp {
		return false
	}
	if claims.ID == "" {
		return true
	}
	grant, ok := utils.ShareAccessGrantsCache.Get(claims.ID)
	if !ok || grant.RemainingUses == 0 {
		return false
	}
	return true
}

// ConsumeDownloadAccessToken decrements use-count for limited tokens.
func ConsumeDownloadAccessToken(tokenParam string) {
	claims, ok := verifySignedToken(tokenParam)
	if !ok || claims == nil || claims.ID == "" {
		return
	}
	grant, ok := utils.ShareAccessGrantsCache.Get(claims.ID)
	if !ok || grant.RemainingUses <= 0 {
		return
	}
	grant.RemainingUses--
	if grant.RemainingUses <= 0 {
		utils.ShareAccessGrantsCache.Delete(claims.ID)
		return
	}
	utils.ShareAccessGrantsCache.Set(claims.ID, grant)
}

// ShareRouteAllowsDownloadToken reports whether an ephemeral download token may authorize this request.
func ShareRouteAllowsDownloadToken(r *http.Request) bool {
	if r == nil {
		return false
	}
	path := r.URL.Path
	return strings.Contains(path, "/resources/download") ||
		strings.Contains(path, "/resources/view") ||
		strings.Contains(path, "/media/stream") ||
		strings.Contains(path, "/raw")
}

// DirectDownloadURL builds a public download URL with an ephemeral token.
func DirectDownloadURL(host, scheme, hash, token string) string {
	tokenParam := ""
	if token != "" {
		tokenParam = fmt.Sprintf("&token=%s", token)
	}
	if settings.Config.Http.ExternalUrl != "" {
		return fmt.Sprintf("%s%spublic/api/resources/download?hash=%s%s",
			settings.Config.Http.ExternalUrl, settings.Config.Http.BaseURL, hash, tokenParam)
	}
	return fmt.Sprintf("%s://%s%spublic/api/resources/download?hash=%s%s",
		scheme, host, settings.Config.Http.BaseURL, hash, tokenParam)
}
