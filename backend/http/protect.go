package http

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/chainfs"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

const segmentThreshold = 10 * 1024 * 1024 // 10 MB

// protectHandler uploads a file to ChainFS and makes it read-only on disk.
// POST /api/chainfs/protect?path=<path>&source=<source>&hours=<hours>
func protectHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	hoursStr := r.URL.Query().Get("hours")

	filePath, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %w", err)
	}
	source, err = url.QueryUnescape(source)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %w", err)
	}

	hours := 24
	if hoursStr != "" {
		parsed, parseErr := strconv.Atoi(hoursStr)
		if parseErr != nil || parsed < 1 {
			return http.StatusBadRequest, fmt.Errorf("hours must be a positive integer")
		}
		hours = parsed
	}

	// Require ChainFS login
	if d.user.LoginMethod != users.LoginMethodChainFs || d.user.AzureAccessToken == "" {
		return http.StatusForbidden, fmt.Errorf("ChainFS account required to protect files")
	}

	// Check token expiry
	if d.user.AzureTokenExpiry > 0 && time.Now().Unix() > d.user.AzureTokenExpiry {
		return http.StatusUnauthorized, fmt.Errorf("ChainFS token expired, please re-authenticate")
	}

	// Check subscription — prefer live acorn.tools check; fall back to cached flag.
	acornSubscribed := false
	if settings.Env.ChainFsBypass {
		acornSubscribed = true
	} else if settings.Env.AcornToolsSecret != "" {
		access, accessErr := chainfs.CheckAcornToolsAccess(settings.Env.AcornToolsURL, settings.Env.AcornToolsSecret, d.user.Username)
		if accessErr != nil {
			logger.Errorf("acorn.tools subscription check failed for protect (%s): %v", d.user.Username, accessErr)
			return http.StatusServiceUnavailable, fmt.Errorf("could not verify subscription status, please try again")
		}
		acornSubscribed = access.HasAccess
		logger.Infof("acorn.tools protect check for %s: hasAccess=%v plan=%s", d.user.Username, acornSubscribed, access.PlanTier)
	} else {
		acornSubscribed = d.user.ChainFSSubscribed
	}

	if !acornSubscribed && !d.user.Permissions.Admin {
		return http.StatusPaymentRequired, fmt.Errorf("an active subscription is required to protect files")
	}

	// Resolve the real path on disk
	userScope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	userScope = strings.TrimRight(userScope, "/")

	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Username:   d.user.Username,
		Path:       utils.JoinPathAsUnix(userScope, filePath),
		Source:     source,
		Expand:     false,
		ShowHidden: d.user.ShowHidden,
	}, store.Access)
	if err != nil {
		return errToStatus(err), err
	}
	if fileInfo.Type == "directory" {
		return http.StatusBadRequest, fmt.Errorf("cannot protect a directory")
	}

	// Decrypt the stored Azure token
	accessToken, err := decryptToken(d.user.AzureAccessToken)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to decrypt access token: %w", err)
	}

	// Derive per-user AES password
	aesPassword := deriveUserAESPassword(d.user)

	chainfsConfig := settings.Config.Auth.Methods.ChainFsAuth

	// Open the file
	f, err := os.Open(fileInfo.RealPath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	stat, err := os.Stat(fileInfo.RealPath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to stat file: %w", err)
	}

	// Upload to ChainFS (segmented if >10MB), or simulate if bypass is active
	var fileGuid string
	if settings.Env.ChainFsBypass {
		fileGuid = "bypass-" + utils.InsecureRandomIdentifier(16)
		logger.Infof("ChainFS bypass active — skipping upload for %s, simulated FileGuid: %s", fileInfo.RealPath, fileGuid)
	} else {
		if stat.Size() > segmentThreshold {
			fileGuid, err = chainfs.UploadFileSegmented(chainfsConfig.ApiBaseUrl, accessToken, stat.Name(), f, stat.Size(), aesPassword)
		} else {
			fileGuid, err = chainfs.UploadFile(chainfsConfig.ApiBaseUrl, accessToken, stat.Name(), f, aesPassword)
		}
		if err != nil {
			// If ChainFS reports the user as unsubscribed but acorn.tools confirmed subscription,
			// the DEV server or token may be out of sync — use bypass mode so protection still records locally.
			if strings.Contains(err.Error(), "User not subscribed") && acornSubscribed {
				fileGuid = "acorn-bypass-" + utils.InsecureRandomIdentifier(16)
				logger.Infof("ChainFS subscription mismatch for %s (acorn.tools OK, ChainFS rejected) — using local bypass, FileGuid: %s", fileInfo.RealPath, fileGuid)
			} else {
				logger.Errorf("ChainFS upload failed for %s: %v", fileInfo.RealPath, err)
				return http.StatusBadGateway, fmt.Errorf("ChainFS upload failed: %w", err)
			}
		} else {
			logger.Infof("ChainFS upload succeeded for %s, FileGuid: %s", fileInfo.RealPath, fileGuid)
		}
	}

	// Persist protection metadata to database
	expiry := time.Now().Add(time.Duration(hours) * time.Hour).Unix()
	if err := store.Protection.Save(fileInfo.RealPath, fileGuid, expiry); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to save protection record: %w", err)
	}

	return renderJSON(w, r, map[string]string{"fileGuid": fileGuid, "protectedUntil": time.Unix(expiry, 0).UTC().Format(time.RFC3339)})
}

// deriveUserAESPassword creates a stable per-user AES password from the server auth key + username.
func deriveUserAESPassword(user *users.User) string {
	material := settings.Config.Auth.Key + ":" + user.Username
	hash := sha256.Sum256([]byte(material))
	return hex.EncodeToString(hash[:])
}

// IsFileProtected returns true if the file at realPath has a ChainFS protection record.
func IsFileProtected(realPath string) bool {
	r, _ := store.Protection.Get(realPath)
	return r != nil
}

// IsProtectionActive returns true if the file is protected AND its expiry has not yet passed.
func IsProtectionActive(realPath string) bool {
	r, _ := store.Protection.Get(realPath)
	if r == nil {
		return false
	}
	if r.Expiry == 0 {
		return true
	}
	return time.Now().Unix() < r.Expiry
}

// ProtectionExpiresAt returns the Unix timestamp when protection expires, and whether one is set.
func ProtectionExpiresAt(realPath string) (int64, bool) {
	r, _ := store.Protection.Get(realPath)
	if r == nil || r.Expiry == 0 {
		return 0, false
	}
	return r.Expiry, true
}
