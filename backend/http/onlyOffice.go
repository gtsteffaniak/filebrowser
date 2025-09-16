package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

const (
	onlyOfficeStatusDocumentClosedWithChanges       = 2
	onlyOfficeStatusDocumentClosedWithNoChanges     = 4
	onlyOfficeStatusForceSaveWhileDocumentStillOpen = 6
)

type OnlyOfficeCallback struct {
	ChangesURL string   `json:"changesurl,omitempty"`
	Key        string   `json:"key,omitempty"`
	Status     int      `json:"status,omitempty"`
	URL        string   `json:"url,omitempty"`
	Users      []string `json:"users,omitempty"`
	UserData   string   `json:"userdata,omitempty"`
}

// OnlyOfficeJWTPayload represents the JWT payload structure for OnlyOffice callbacks
type OnlyOfficeJWTPayload struct {
	Key     string   `json:"key"`
	Status  int      `json:"status"`
	Users   []string `json:"users"`
	Actions []struct {
		Type   int    `json:"type"`
		UserID string `json:"userid"`
	} `json:"actions"`
}

func onlyofficeClientConfigGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if settings.Config.Integrations.OnlyOffice.Url == "" {
		return http.StatusInternalServerError, errors.New("only-office integration must be configured in settings")
	}

	// Extract clean parameters from request
	source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")

	// Validate required parameters
	if (path == "" || source == "") && d.fileInfo.Hash == "" {
		logger.Errorf("OnlyOffice callback missing required parameters: source=%s, path=%s", source, path)
		return http.StatusBadRequest, errors.New("missing required parameters: path + source/hash are required")
	}
	themeMode := utils.Ternary(d.user.DarkMode, "dark", "light")
	var sourceInfo settings.Source
	var ok bool
	if d.fileInfo.Hash != "" {
		sourceInfo, ok = settings.Config.Server.SourceMap[source]
		if !ok {
			logger.Error("OnlyOffice: source not found")
			return http.StatusInternalServerError, fmt.Errorf("source not found")
		}
	} else {
		sourceInfo, ok = settings.Config.Server.NameToSource[source]
		if !ok {
			logger.Error("OnlyOffice: source not found")
			return http.StatusInternalServerError, fmt.Errorf("source not found")
		}
	}
	if d.fileInfo.Hash == "" {
		// Build file info based on whether this is a share or regular request
		// Regular user request - need to resolve scope
		userScope, scopeErr := settings.GetScopeFromSourceName(d.user.Scopes, source)
		if scopeErr != nil {
			logger.Errorf("OnlyOffice: source %s not available for user %s: %v", source, d.user.Username, scopeErr)
			return http.StatusForbidden, fmt.Errorf("source %s is not available", source)
		}
		indexPath := utils.JoinPathAsUnix(userScope, path)
		logger.Debugf("OnlyOffice user request: resolved path=%s", indexPath)
		fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
			Path:   indexPath,
			Modify: d.user.Permissions.Modify,
			Source: source,
			Expand: false,
		})
		if err != nil {
			logger.Errorf("OnlyOffice: failed to get file info for source=%s, path=%s: %v", source, indexPath, err)
			return errToStatus(err), err
		}
		d.fileInfo = *fileInfo
	} else {
		source = sourceInfo.Name
		// path is index path, so we build from share path
		path = utils.JoinPathAsUnix(d.share.Path, path)
		if d.share.EnforceDarkLightMode == "dark" {
			themeMode = "dark"
		}
		if d.share.EnforceDarkLightMode == "light" {
			themeMode = "light"
		}

	}

	// Determine file type and editing permissions
	fileType := strings.TrimPrefix(filepath.Ext(d.fileInfo.Name), ".")
	canEdit := iteminfo.CanEditOnlyOffice(d.user.Permissions.Modify, fileType)
	canEditMode := utils.Ternary(canEdit, "edit", "view")
	if d.fileInfo.Hash != "" {
		if d.share.EnableOnlyOfficeEditing {
			canEditMode = "edit"
		}
	}
	// For shares, we need to keep track of the original relative path for the callback URL
	var callbackPath string
	if d.fileInfo.Hash != "" {
		// For shares, use the original path parameter (relative to share)
		callbackPath = r.URL.Query().Get("path")
	} else {
		// For regular requests, use the processed path
		callbackPath = path
	}

	// Generate document ID for OnlyOffice
	documentId, err := getOnlyOfficeId(d.fileInfo.RealPath)
	if err != nil {
		logger.Errorf("OnlyOffice: failed to generate document ID for source=%s, path=%s: %v", source, path, err)
		return http.StatusNotFound, fmt.Errorf("failed to generate document ID: %v", err)
	}

	// Build download URL that OnlyOffice server will use
	downloadURL := buildOnlyOfficeDownloadURL(source, callbackPath, d.fileInfo.Hash, d.token)

	// Build callback URL for OnlyOffice to notify us of changes
	callbackURL := buildOnlyOfficeCallbackURL(source, callbackPath, d.fileInfo.Hash, d.token)

	// Build OnlyOffice client configuration
	clientConfig := map[string]interface{}{
		"document": map[string]interface{}{
			"fileType": fileType,
			"key":      documentId,
			"title":    d.fileInfo.Name,
			"url":      downloadURL,
			"permissions": map[string]interface{}{
				"edit":     utils.Ternary(settings.Config.Integrations.OnlyOffice.ViewOnly, "view", canEditMode),
				"download": true,
				"print":    true,
			},
		},
		"editorConfig": map[string]interface{}{
			"callbackUrl": callbackURL,
			"user": map[string]interface{}{
				"id":   strconv.FormatUint(uint64(d.user.ID), 10),
				"name": d.user.Username,
			},
			"customization": map[string]interface{}{
				"autosave":  true,
				"forcesave": true,
				"uiTheme":   themeMode,
			},
			"lang": d.user.Locale,
			"mode": utils.Ternary(settings.Config.Integrations.OnlyOffice.ViewOnly, "view", canEditMode),
		},
	}

	// Sign configuration with JWT if secret is configured
	if settings.Config.Integrations.OnlyOffice.Secret != "" {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(clientConfig))
		signature, err := token.SignedString([]byte(settings.Config.Integrations.OnlyOffice.Secret))
		if err != nil {
			logger.Errorf("OnlyOffice: failed to sign JWT: %v", err)
			return http.StatusInternalServerError, fmt.Errorf("failed to sign configuration")
		}
		clientConfig["token"] = signature
	}

	return renderJSON(w, r, clientConfig)
}

// buildOnlyOfficeDownloadURL constructs the download URL that OnlyOffice server will use to fetch the file
func buildOnlyOfficeDownloadURL(source, path, hash, token string) string {
	// Determine base URL (internal URL takes priority for OnlyOffice server communication)
	baseURL := settings.Config.Server.BaseURL
	if settings.Config.Server.InternalUrl != "" {
		// Ensure proper URL joining without double slashes
		internalURL := strings.TrimSuffix(settings.Config.Server.InternalUrl, "/")
		baseURLPath := strings.TrimPrefix(settings.Config.Server.BaseURL, "/")
		baseURL = internalURL + "/" + baseURLPath
	}

	var downloadURL string
	if hash != "" {
		// Share download URL - don't expose source name, just use the path relative to share
		filesParam := url.QueryEscape(path)
		downloadURL = fmt.Sprintf("%s/public/api/raw?files=%s&hash=%s&token=%s&auth=%s",
			strings.TrimSuffix(baseURL, "/"), filesParam, hash, token, token)
	} else {
		// Regular download URL - include source for non-share requests
		filesParam := url.QueryEscape(source + "::" + path)
		downloadURL = fmt.Sprintf("%s/api/raw?files=%s&auth=%s",
			strings.TrimSuffix(baseURL, "/"), filesParam, token)
	}

	return downloadURL
}

// buildOnlyOfficeCallbackURL constructs the callback URL that OnlyOffice server will use to notify us of changes
func buildOnlyOfficeCallbackURL(source, path, hash, token string) string {
	baseURL := settings.Config.Server.BaseURL
	if settings.Config.Server.InternalUrl != "" {
		// Ensure proper URL joining without double slashes
		internalURL := strings.TrimSuffix(settings.Config.Server.InternalUrl, "/")
		baseURLPath := strings.TrimPrefix(settings.Config.Server.BaseURL, "/")
		baseURL = internalURL + "/" + baseURLPath
	}

	var callbackURL string
	if hash != "" {
		// Share callback URL - use public API and don't expose source, use path relative to share
		params := url.Values{}
		params.Set("hash", hash)
		params.Set("path", path) // This should be the path relative to the share, not the full filesystem path
		params.Set("auth", token)

		callbackURL = fmt.Sprintf("%s/public/api/onlyoffice/callback?%s",
			strings.TrimSuffix(baseURL, "/"), params.Encode())
	} else {
		// Regular callback URL - include source for non-share requests
		params := url.Values{}
		params.Set("source", source)
		params.Set("path", path)
		params.Set("auth", token)

		callbackURL = fmt.Sprintf("%s/api/onlyoffice/callback?%s",
			strings.TrimSuffix(baseURL, "/"), params.Encode())
	}

	return callbackURL
}

// processOnlyOfficeCallback handles the common callback processing logic for both GET and POST requests
func processOnlyOfficeCallback(w http.ResponseWriter, r *http.Request, d *requestContext, data *OnlyOfficeCallback) (int, error) {
	// Extract clean parameters from query string
	source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")

	// Validate required parameters
	if (path == "" || source == "") && d.fileInfo.Hash == "" {
		logger.Errorf("OnlyOffice callback missing required parameters: source=%s, path=%s", source, path)
		return http.StatusBadRequest, errors.New("missing required parameters: path + source/hash are required")
	}
	var sourceInfo settings.Source
	var ok bool
	if d.fileInfo.Hash != "" {
		sourceInfo, ok = settings.Config.Server.SourceMap[source]
		if !ok {
			logger.Error("OnlyOffice: source not found")
			return http.StatusInternalServerError, fmt.Errorf("source not found")
		}
	} else {
		sourceInfo, ok = settings.Config.Server.NameToSource[source]
		if !ok {
			logger.Error("OnlyOffice: source not found")
			return http.StatusInternalServerError, fmt.Errorf("source not found")
		}
	}

	if d.fileInfo.Hash == "" {
		// Regular user request - need to resolve scope
		userScope, scopeErr := settings.GetScopeFromSourceName(d.user.Scopes, source)
		if scopeErr != nil {
			logger.Errorf("OnlyOffice callback: source %s not available for user %s: %v", source, d.user.Username, scopeErr)
			return http.StatusForbidden, fmt.Errorf("source %s is not available", source)
		}
		path = utils.JoinPathAsUnix(userScope, path)
	} else {
		source = sourceInfo.Name
		// path is index path, so we build from share path
		path = utils.JoinPathAsUnix(d.share.Path, path)
	}
	// Handle document closure - clean up document key cache
	if data.Status == onlyOfficeStatusDocumentClosedWithChanges ||
		data.Status == onlyOfficeStatusDocumentClosedWithNoChanges {
		// Refer to OnlyOffice documentation:
		// - https://api.onlyoffice.com/editors/coedit
		// - https://api.onlyoffice.com/editors/callback
		//
		// When the document is fully closed by all editors,
		// the document key should no longer be re-used.
		deleteOfficeId(source, path)
	}

	// Handle document save operations
	if data.Status == onlyOfficeStatusDocumentClosedWithChanges ||
		data.Status == onlyOfficeStatusForceSaveWhileDocumentStillOpen {

		// Verify user has modify permissions
		if !d.user.Permissions.Modify {
			logger.Warningf("OnlyOffice callback: user %s lacks modify permissions for source=%s, path=%s",
				d.user.Username, source, path)
			return http.StatusForbidden, nil
		}

		// Download the updated document from OnlyOffice server
		doc, err := http.Get(data.URL)
		if err != nil {
			logger.Errorf("OnlyOffice callback: failed to download updated document: %v", err)
			return http.StatusInternalServerError, err
		}
		defer doc.Body.Close()

		// Resolve file path for writing (same logic as in config handler)
		var resolvedPath string
		if d.fileInfo.Hash == "" {
			// Regular user request - need to resolve scope
			userScope, scopeErr := settings.GetScopeFromSourceName(d.user.Scopes, source)
			if scopeErr != nil {
				logger.Errorf("OnlyOffice callback: source %s not available for user %s: %v",
					source, d.user.Username, scopeErr)
				return http.StatusForbidden, fmt.Errorf("source %s is not available", source)
			}
			resolvedPath = utils.JoinPathAsUnix(userScope, path)
		}

		// Write the updated document
		fileOpts := iteminfo.FileOptions{
			Path:   resolvedPath,
			Source: source,
		}
		writeErr := files.WriteFile(fileOpts, doc.Body)
		if writeErr != nil {
			logger.Errorf("OnlyOffice callback: failed to write updated document: %v", writeErr)
			return http.StatusInternalServerError, writeErr
		}

	}

	// Return success response to OnlyOffice server
	resp := map[string]int{
		"error": 0,
	}
	return renderJSON(w, r, resp)
}

func onlyofficeCallbackHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Parse callback data based on request method
	var callbackData *OnlyOfficeCallback
	var err error

	if r.Method == "GET" {
		// OnlyOffice sends callback data in Authorization header as JWT
		callbackData, err = parseOnlyOfficeCallbackFromJWT(r)
	} else if r.Method == "POST" {
		// OnlyOffice sends callback data in request body as JSON
		callbackData, err = parseOnlyOfficeCallbackFromJSON(r)
	} else {
		return http.StatusMethodNotAllowed, fmt.Errorf("unsupported method: %s", r.Method)
	}

	if err != nil {
		logger.Errorf("OnlyOffice callback: failed to parse callback data: %v", err)
		return http.StatusBadRequest, err
	}

	// Process the callback data using shared logic
	return processOnlyOfficeCallback(w, r, d, callbackData)
}

// parseOnlyOfficeCallbackFromJWT extracts callback data from JWT in Authorization header
func parseOnlyOfficeCallbackFromJWT(r *http.Request) (*OnlyOfficeCallback, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, errors.New("missing Authorization header")
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, errors.New("invalid Authorization header format")
	}

	jwtToken := strings.TrimPrefix(authHeader, "Bearer ")

	return parseOnlyOfficeJWT(jwtToken)
}

// parseOnlyOfficeCallbackFromJSON extracts callback data from JSON request body
func parseOnlyOfficeCallbackFromJSON(r *http.Request) (*OnlyOfficeCallback, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %v", err)
	}
	var data OnlyOfficeCallback
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return &data, nil
}

func getOnlyOfficeId(realpath string) (string, error) {
	// error is intentionally ignored in order treat errors
	// the same as a cache-miss
	cachedDocumentKey, ok := utils.OnlyOfficeCache.Get(realpath).(string)
	if ok {
		return cachedDocumentKey, nil
	}
	return "", fmt.Errorf("document key not found")
}

func deleteOfficeId(source, path string) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		logger.Errorf("deleteOfficeId: failed to find source index for user home dir creation: %s", source)
		return
	}
	realpath, _, _ := idx.GetRealPath(path)
	utils.OnlyOfficeCache.Delete(realpath)
}

// parseOnlyOfficeJWT parses the JWT token from OnlyOffice callback
func parseOnlyOfficeJWT(tokenString string) (*OnlyOfficeCallback, error) {
	// Parse the JWT token without signature verification since OnlyOffice uses different signing
	// We'll parse it manually to avoid signature validation issues
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part) with fallback to standard base64
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Fallback to standard base64 decoding
		payloadBytes, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("failed to decode JWT payload: %v", err)
		}
	}

	var claims jwt.MapClaims
	err = json.Unmarshal(payloadBytes, &claims)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT claims: %v", err)
	}

	// Extract payload from claims with fallback
	payload, ok := claims["payload"].(map[string]interface{})
	if !ok {
		// Fallback: try to use claims directly if no payload wrapper
		payload = map[string]interface{}(claims)
	}

	// Convert to OnlyOfficeCallback struct with safe type assertions
	callback := &OnlyOfficeCallback{}

	// Extract key with validation
	if key, ok := payload["key"].(string); ok && key != "" {
		callback.Key = key
	} else {
		logger.Warningf("OnlyOffice callback: missing or empty key in JWT payload")
	}

	// Extract status with validation
	if status, ok := payload["status"].(float64); ok {
		callback.Status = int(status)
	} else {
		logger.Warningf("OnlyOffice callback: missing or invalid status in JWT payload")
		callback.Status = 0 // Default to unknown status
	}

	// Extract users with safe array handling
	if users, ok := payload["users"].([]interface{}); ok {
		callback.Users = make([]string, 0, len(users))
		for _, user := range users {
			if userStr, ok := user.(string); ok && userStr != "" {
				callback.Users = append(callback.Users, userStr)
			}
		}
	}

	// Validate essential fields
	if callback.Key == "" {
		return nil, fmt.Errorf("missing document key in JWT payload")
	}

	return callback, nil
}
