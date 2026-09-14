package web

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const (
	shareAccessScopeDownload = "download"
	shareAccessScopeUISession  = "ui"
	maxShareAccessTTL          = 24 * time.Hour
)

// shareDownloadGrantMu serializes use-count decrements for limited download tokens.
var shareDownloadGrantMu sync.Mutex

type shareAccessClaims struct {
	Hash  string `json:"h"`
	Exp   int64  `json:"exp"`
	Scope string `json:"s"`
	ID    string `json:"id,omitempty"`
}

// parseShareAccessDuration converts API duration parameters into a time.Duration.
func parseShareAccessDuration(duration int, unit string) (time.Duration, error) {
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

// parseShareDownloadAccessDuration validates share direct-download TTL capped at 24 hours.
func parseShareDownloadAccessDuration(duration int, unit string) (time.Duration, error) {
	ttl, err := parseShareAccessDuration(duration, unit)
	if err != nil {
		return 0, err
	}
	if ttl > maxShareAccessTTL {
		return 0, fmt.Errorf("duration exceeds maximum of 24 hours")
	}
	return ttl, nil
}

// shareAccessAuthKey returns the HMAC signing key for share access tokens.
func shareAccessAuthKey() []byte {
	return []byte(settings.Config.Auth.Key)
}

// mintSignedShareAccessToken signs share access claims as a base64url.payload.signature token.
func mintSignedShareAccessToken(claims shareAccessClaims) (token string, expiresAt int64, err error) {
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", 0, err
	}
	return utils.SignHMACSHA256Base64URL(shareAccessAuthKey(), payload), claims.Exp, nil
}

// verifySignedShareAccessToken parses and verifies a signed share access token payload.
func verifySignedShareAccessToken(tokenParam string) (*shareAccessClaims, bool) {
	payload, ok := utils.VerifyHMACSHA256Base64URL(shareAccessAuthKey(), tokenParam)
	if !ok {
		return nil, false
	}
	var claims shareAccessClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, false
	}
	return &claims, true
}

// mintShareDownloadAccessToken creates a signed, download-scoped ephemeral token for a share.
func mintShareDownloadAccessToken(shareHash string, ttl time.Duration, maxUses int) (token string, expiresAt int64, err error) {
	if shareHash == "" {
		return "", 0, fmt.Errorf("share hash is required")
	}
	if ttl <= 0 || ttl > maxShareAccessTTL {
		return "", 0, fmt.Errorf("invalid token duration")
	}
	if maxUses < 0 {
		return "", 0, fmt.Errorf("invalid max uses")
	}

	expiresAt = time.Now().Add(ttl).Unix()
	claims := shareAccessClaims{
		Hash:  shareHash,
		Exp:   expiresAt,
		Scope: shareAccessScopeDownload,
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

	token, expiresAt, err = mintSignedShareAccessToken(claims)
	if err != nil {
		return "", 0, err
	}

	if claims.ID != "" {
		utils.ShareAccessGrantsCache.Set(claims.ID, utils.ShareAccessGrant{RemainingUses: remainingUses})
	}

	return token, expiresAt, nil
}

// mintShareUISessionToken creates a signed cookie-scoped token for UI share access after password entry.
func mintShareUISessionToken(shareHash string, ttl time.Duration) (token string, expiresAt int64, err error) {
	if shareHash == "" {
		return "", 0, fmt.Errorf("share hash is required")
	}
	if ttl <= 0 || ttl > maxShareAccessTTL {
		return "", 0, fmt.Errorf("invalid token duration")
	}

	expiresAt = time.Now().Add(ttl).Unix()
	return mintSignedShareAccessToken(shareAccessClaims{
		Hash:  shareHash,
		Exp:   expiresAt,
		Scope: shareAccessScopeUISession,
	})
}

// validateShareUISessionToken checks a UI session token for the given share hash.
func validateShareUISessionToken(tokenParam, shareHash string) bool {
	claims, ok := verifySignedShareAccessToken(tokenParam)
	if !ok || claims == nil {
		return false
	}
	if claims.Scope != shareAccessScopeUISession || claims.Hash != shareHash {
		return false
	}
	return time.Now().Unix() <= claims.Exp
}

// validateShareDownloadAccessToken checks a download token without consuming a limited use.
func validateShareDownloadAccessToken(tokenParam, shareHash string) bool {
	claims, ok := verifySignedShareAccessToken(tokenParam)
	if !ok || claims == nil {
		return false
	}
	if claims.Scope != shareAccessScopeDownload || claims.Hash != shareHash {
		return false
	}
	if time.Now().Unix() > claims.Exp {
		return false
	}
	if claims.ID == "" {
		return true
	}
	grant, ok := utils.ShareAccessGrantsCache.Get(claims.ID)
	if !ok || grant.RemainingUses <= 0 {
		return false
	}
	return true
}

// authorizeShareDownloadAccessToken validates a download token and atomically consumes one use when limited.
func authorizeShareDownloadAccessToken(tokenParam, shareHash string) bool {
	claims, ok := verifySignedShareAccessToken(tokenParam)
	if !ok || claims == nil {
		return false
	}
	if claims.Scope != shareAccessScopeDownload || claims.Hash != shareHash {
		return false
	}
	if time.Now().Unix() > claims.Exp {
		return false
	}
	if claims.ID == "" {
		return true
	}

	shareDownloadGrantMu.Lock()
	defer shareDownloadGrantMu.Unlock()

	grant, ok := utils.ShareAccessGrantsCache.Get(claims.ID)
	if !ok || grant.RemainingUses <= 0 {
		return false
	}
	grant.RemainingUses--
	if grant.RemainingUses <= 0 {
		utils.ShareAccessGrantsCache.Delete(claims.ID)
	} else {
		utils.ShareAccessGrantsCache.Set(claims.ID, grant)
	}
	return true
}

// shareRouteAllowsDownloadToken reports whether an ephemeral download token may authorize this path.
func shareRouteAllowsDownloadToken(path string) bool {
	return strings.Contains(path, "/resources/download") ||
		strings.Contains(path, "/resources/view") ||
		strings.Contains(path, "/media/stream") ||
		strings.Contains(path, "/raw")
}

// shareRequestAllowsDownloadToken reports whether an ephemeral download token may authorize this request.
func shareRequestAllowsDownloadToken(r *http.Request) bool {
	if r == nil {
		return false
	}
	return shareRouteAllowsDownloadToken(r.URL.Path)
}

// directDownloadURL builds a public download URL with an ephemeral token query parameter.
func directDownloadURL(host, scheme, hash, token string) string {
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
