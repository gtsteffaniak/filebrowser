package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

const (
	onlyOfficeStatusDocumentBeingEdited             = 1
	onlyOfficeStatusDocumentClosedWithChanges       = 2
	onlyOfficeStatusDocumentSavingError             = 3
	onlyOfficeStatusDocumentClosedWithNoChanges     = 4
	onlyOfficeStatusForceSaveWhileDocumentStillOpen = 6
	onlyOfficeStatusForceSaveError                  = 7

	onlyOfficeDownloadTimeout = 10 * time.Second
)

// onlyOfficeDownloadClient fetches saved documents from the OnlyOffice document server.
// A bounded timeout avoids hanging goroutines when the server is unreachable.
var onlyOfficeDownloadClient = &http.Client{
	Timeout: onlyOfficeDownloadTimeout,
}

type OnlyOfficeCallback struct {
	Actions       []OnlyOfficeAction `json:"actions,omitempty"`
	ChangesURL    string             `json:"changesurl,omitempty"`
	FileType      string             `json:"filetype,omitempty"`
	ForceSaveType int                `json:"forcesavetype,omitempty"`
	FormsDataURL  string             `json:"formsdataurl,omitempty"`
	History       *OnlyOfficeHistory `json:"history,omitempty"`
	Key           string             `json:"key,omitempty"`
	Status        int                `json:"status,omitempty"`
	URL           string             `json:"url,omitempty"`
	UserData      string             `json:"userdata,omitempty"`
	Users         []string           `json:"users,omitempty"`
}

type OnlyOfficeAction struct {
	Type   int    `json:"type"`
	UserID string `json:"userid"`
}

type OnlyOfficeHistory struct {
	Changes       interface{} `json:"changes"`
	ServerVersion string      `json:"serverVersion"`
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

// onlyofficeClientConfigGetHandler retrieves OnlyOffice client configuration
//
// @Summary Get OnlyOffice client configuration
// @Description Returns the configuration needed for OnlyOffice document editor client
// @Tags Office
// @Accept json
// @Produce json
// @Param source query string false "Source name"
// @Param path query string false "File path"
// @Param hash query string false "Share hash (for public shares)"
// @Success 200 {object} map[string]interface{} "OnlyOffice configuration"
// @Failure 400 {object} map[string]string "Missing or invalid parameters"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/office/config [get]
// @Security ApiKeyAuth
func onlyofficeClientConfigGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if settings.Config.Integrations.OnlyOffice.Url == "" {
		return http.StatusInternalServerError, errors.New("only-office integration must be configured in settings")
	}
	if !onlyOfficeShareEnabled(d) {
		return http.StatusForbidden, errors.New("onlyoffice is not enabled for this share")
	}

	// Extract clean parameters from request
	source := r.URL.Query().Get("source")
	providedPath := r.URL.Query().Get("path")
	hash := r.URL.Query().Get("hash")

	// Validate required parameters
	if (providedPath == "" || source == "") && hash == "" {
		logger.Errorf("OnlyOffice callback missing required parameters: source=%s, path=%s", source, providedPath)
		return http.StatusBadRequest, errors.New("missing required parameters: path + source/hash are required")
	}

	// Rule 1: Validate user-provided path to prevent path traversal
	cleanPath, err := utils.SanitizePath(providedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}
	providedPath = cleanPath

	themeMode := utils.Ternary(d.User.DarkMode, "dark", "light")
	var sourceInfo *settings.Source
	var ok bool
	if hash != "" {
		sourceInfo, ok = settings.Config.Server.SourceMap[d.Share.SourcePath]
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
	source = sourceInfo.Name
	path := providedPath
	if hash == "" {
		// Build file info based on whether this is a share or regular request
		// Regular user request
		logger.Debugf("OnlyOffice user request: request path=%s", path)
		var fileInfo *iteminfo.ExtendedFileInfo
		fileInfo, err = files.FileInfoFaster(utils.FileOptions{
			Path:           path,
			Source:         source,
			Expand:         false,
			FollowSymlinks: true,
		}, d.User)
		if err != nil {
			logger.Errorf("OnlyOffice: failed to get file info for source=%s, path=%s: %v", source, path, err)
			return ErrToStatus(err), err
		}
		d.FileInfo = *fileInfo
	} else {
		// path is index path, so we build from share path
		path = utils.JoinPathAsUnix(d.Share.Path, providedPath)
		if d.Share.EnforceDarkLightMode == "dark" {
			themeMode = "dark"
		}
		if d.Share.EnforceDarkLightMode == "light" {
			themeMode = "light"
		}

	}

	// Determine file type and editing permissions
	fileType := strings.TrimPrefix(filepath.Ext(d.FileInfo.Name), ".")

	filePerms, permErr := effectiveFilePerms(d, source)
	if permErr != nil {
		return http.StatusForbidden, permErr
	}
	if !filePerms.View {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to view files in this source")
	}
	modifyPerms := filePerms.Modify
	allowDownload := filePerms.Download
	allowPrint := filePerms.View

	canEdit := iteminfo.CanEditOnlyOffice(modifyPerms, fileType)
	canEditMode := utils.Ternary(canEdit, "edit", "view")
	// Generate document ID for OnlyOffice
	documentId, err := GetOnlyOfficeId(d.FileInfo.RealPath)
	if err != nil {
		logger.Errorf("OnlyOffice: failed to generate document ID for source=%s, path=%s: %v", source, path, err)
		return http.StatusNotFound, fmt.Errorf("failed to generate document ID: %v", err)
	}

	// Create and store log context for this OnlyOffice session
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		sessionID = "unknown"
	}

	shareHash := ""
	if d.FileInfo.Hash != "" {
		shareHash = d.FileInfo.Hash
	}

	logContext := CreateOnlyOfficeLogContext(
		d.User.Username,
		sessionID,
		documentId,
		path,
		source,
		shareHash,
		d.User.Permissions.Admin,
	)
	StoreOnlyOfficeLogContext(documentId, logContext)

	// Send initial log event with detailed path information
	SendOnlyOfficeLogEvent(logContext, "INFO", "config", fmt.Sprintf("OnlyOffice session started for document: %s ", path))

	// Mint source-scoped view grant for OnlyOffice document fetch (viewing, not download)
	viewToken, err := mintViewGrant(d, source)
	if err != nil {
		logger.Errorf("OnlyOffice: failed to mint view grant: %v", err)
		return http.StatusForbidden, err
	}

	shareToken := shareTokenForOnlyOffice(d)

	// Build view URL that OnlyOffice server will use to fetch the file
	documentURL := buildOnlyOfficeViewURL(r, source, providedPath, d.FileInfo.Hash, viewToken, d.Token, shareToken)

	// Build callback URL for OnlyOffice to notify us of changes
	callbackURL := buildOnlyOfficeCallbackURL(r, source, providedPath, d.FileInfo.Hash, d.Token, shareToken)

	// Build OnlyOffice client configuration
	clientConfig := map[string]interface{}{
		"document": map[string]interface{}{
			"fileType": fileType,
			"key":      documentId,
			"title":    d.FileInfo.Name,
			"url":      documentURL,
			"permissions": map[string]interface{}{
				"edit":     utils.Ternary(settings.Config.Integrations.OnlyOffice.ViewOnly, "view", canEditMode),
				"download": allowDownload,
				"print":    allowPrint,
			},
		},
		"editorConfig": map[string]interface{}{
			"callbackUrl": callbackURL,
			"user": map[string]interface{}{
				"id":   strconv.FormatUint(uint64(d.User.ID), 10),
				"name": d.User.Username,
			},
			"customization": map[string]interface{}{
				"autosave":  true,
				"forcesave": true,
				"uiTheme":   themeMode,
			},
			"lang": d.User.Locale,
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

	return RenderJSON(w, r, clientConfig)
}

// onlyOfficeFileBrowserBaseURL returns the base URL OnlyOffice uses to reach FileBrowser.
// Priority: http.internalUrl → http.externalUrl → incoming request (with trustProxyHeaders).
// The result always ends with a single trailing slash when a base path is configured.
func onlyOfficeFileBrowserBaseURL(r *http.Request) string {
	for _, configured := range []string{
		settings.Config.Http.InternalUrl,
		settings.Config.Http.ExternalUrl,
	} {
		if configured == "" {
			continue
		}
		root := strings.TrimSuffix(configured, "/")
		baseURLPath := strings.Trim(settings.Config.Http.BaseURL, "/")
		if baseURLPath != "" {
			return root + "/" + baseURLPath + "/"
		}
		return root + "/"
	}
	host := requestHost(r)
	scheme := requestSchemeForPublicURL(r)
	baseURLPath := strings.Trim(settings.Config.Http.BaseURL, "/")
	if baseURLPath == "" {
		return fmt.Sprintf("%s://%s/", scheme, host)
	}
	return fmt.Sprintf("%s://%s/%s/", scheme, host, baseURLPath)
}

// joinOnlyOfficeAPIURL appends an API path to a base URL that may end with "/".
func joinOnlyOfficeAPIURL(baseURL, apiPath string) string {
	base := strings.TrimRight(baseURL, "/")
	path := strings.TrimLeft(apiPath, "/")
	return base + "/" + path
}

// shareTokenForOnlyOffice mints a short-lived download token for OnlyOffice server URLs.
func shareTokenForOnlyOffice(d *Context) string {
	if d.Share.Hash == "" || d.Share.PasswordHash == "" {
		return ""
	}
	token, _, err := mintShareDownloadAccessToken(d.Share.Hash, 2*time.Hour, 0)
	if err != nil {
		logger.Errorf("failed to mint OnlyOffice share download token: hash=%s error=%v", d.Share.Hash, err)
		return ""
	}
	return token
}

// buildOnlyOfficeViewURL constructs the view URL that OnlyOffice server uses to fetch the document.
func buildOnlyOfficeViewURL(r *http.Request, source, path, hash, viewToken, authToken, shareToken string) string {
	baseURL := onlyOfficeFileBrowserBaseURL(r)
	params := url.Values{}
	params.Set("file", path)
	params.Set("viewToken", viewToken)
	if hash != "" {
		params.Set("hash", hash)
		if shareToken != "" {
			params.Set("token", shareToken)
		}
		return joinOnlyOfficeAPIURL(baseURL, "public/api/resources/view") + "?" + params.Encode()
	}
	params.Set("source", source)
	params.Set("auth", authToken)
	return joinOnlyOfficeAPIURL(baseURL, "api/resources/view") + "?" + params.Encode()
}

// buildOnlyOfficeCallbackURL constructs the callback URL that OnlyOffice server will use to notify us of changes
func buildOnlyOfficeCallbackURL(r *http.Request, source, path, hash, authToken, shareToken string) string {
	baseURL := onlyOfficeFileBrowserBaseURL(r)

	params := url.Values{}
	if hash != "" {
		// Share callback URL - use public API and don't expose source, use path relative to share
		params.Set("hash", hash)
		params.Set("path", path)
		params.Set("auth", authToken)
		if shareToken != "" {
			params.Set("token", shareToken)
		}
		return joinOnlyOfficeAPIURL(baseURL, "public/api/office/callback") + "?" + params.Encode()
	}

	// Regular callback URL - include source for non-share requests
	params.Set("source", source)
	params.Set("path", path)
	params.Set("auth", authToken)
	return joinOnlyOfficeAPIURL(baseURL, "api/office/callback") + "?" + params.Encode()
}

// resolveOnlyOfficeDownloadURL validates a callback document URL against
// integrations.office.url and optionally rewrites the origin to integrations.office.internalUrl.
// Returns an empty string when the URL is missing, malformed, or not hosted on the configured
// OnlyOffice server (SSRF protection).
func resolveOnlyOfficeDownloadURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	publicBase := settings.Config.Integrations.OnlyOffice.Url
	if publicBase == "" {
		logger.Warningf("OnlyOffice callback: integrations.office.url is not configured")
		return ""
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		logger.Warningf("OnlyOffice callback: could not parse document URL (%q): %v", rawURL, err)
		return ""
	}

	publicURL, err := url.Parse(publicBase)
	if err != nil {
		logger.Warningf("OnlyOffice callback: could not parse integrations.office.url (%q): %v", publicBase, err)
		return ""
	}

	if !onlyOfficeURLHostsMatch(parsedURL, publicURL) {
		logger.Warningf("OnlyOffice callback: rejecting document URL with untrusted host %q (expected %q)",
			parsedURL.Host, publicURL.Host)
		return ""
	}

	internalBase := settings.Config.Integrations.OnlyOffice.InternalUrl
	if internalBase == "" {
		return rawURL
	}

	internalURL, err := url.Parse(internalBase)
	if err != nil {
		logger.Warningf("OnlyOffice callback: could not parse integrations.office.internalUrl (%q): %v", internalBase, err)
		return ""
	}
	if !isAllowedOnlyOfficeScheme(internalURL.Scheme) || internalURL.Host == "" {
		logger.Warningf("OnlyOffice callback: invalid integrations.office.internalUrl (%q)", internalBase)
		return ""
	}

	rewritten := *parsedURL
	rewritten.Scheme = internalURL.Scheme
	rewritten.Host = internalURL.Host
	result := rewritten.String()
	if result != rawURL {
		logger.Debugf("OnlyOffice callback: rewrote URL from %s to %s", rawURL, result)
	}
	return result
}

func isAllowedOnlyOfficeScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
}

func onlyOfficeEffectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch u.Scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}

// onlyOfficeURLHostsMatch reports whether callback and configured URLs refer to the same
// OnlyOffice host, comparing hostname and effective port (so office.local matches office.local:80).
func onlyOfficeURLHostsMatch(callback, configured *url.URL) bool {
	if !isAllowedOnlyOfficeScheme(callback.Scheme) || !isAllowedOnlyOfficeScheme(configured.Scheme) {
		return false
	}
	if callback.Hostname() == "" || configured.Hostname() == "" {
		return false
	}
	if !strings.EqualFold(callback.Hostname(), configured.Hostname()) {
		return false
	}
	return onlyOfficeEffectivePort(callback) == onlyOfficeEffectivePort(configured)
}

// onlyOfficeShareEnabled reports whether the active request may use OnlyOffice.
// Requests carrying a share context are rejected when the share disables the integration.
func onlyOfficeShareEnabled(d *Context) bool {
	return d.Share.Hash == "" || d.Share.EnableOnlyOffice
}

// processOnlyOfficeCallback handles the common callback processing logic for both GET and POST requests
func processOnlyOfficeCallback(w http.ResponseWriter, r *http.Request, d *Context, data *OnlyOfficeCallback) (int, error) {
	if !onlyOfficeShareEnabled(d) {
		return returnOnlyOfficeError(w, r, 403, "onlyoffice is not enabled for this share")
	}

	// Extract clean parameters from query string
	source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	user := d.User

	if d.Share.Hash != "" {
		source = d.Share.GetSourceName()
		path = d.IndexPath
		user = d.ShareUser
	}

	// Validate required parameters
	if (path == "" || source == "") && d.FileInfo.Hash == "" {
		logger.Errorf("OnlyOffice callback missing required parameters: source=%s, path=%s", source, path)
		return returnOnlyOfficeError(w, r, 400, "missing required parameters: path + source/hash are required")
	}

	// Rule 1: Validate user-provided path to prevent path traversal
	cleanPath, err := utils.SanitizePath(path)
	if err != nil {
		return returnOnlyOfficeError(w, r, 400, err.Error())
	}
	path = cleanPath

	if err := validateOnlyOfficeCallbackKey(source, path, user, data); err != nil {
		return returnOnlyOfficeError(w, r, 400, err.Error())
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
		deleteOfficeId(source, path, user)

		// Send log event for document closure and clean up log context
		if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
			statusMsg := "Document closed with changes"
			if data.Status == onlyOfficeStatusDocumentClosedWithNoChanges {
				statusMsg = "Document closed with no changes"
			}
			SendOnlyOfficeLogEvent(logContext, "INFO", "callback", statusMsg)
			RemoveOnlyOfficeLogContext(data.Key)
		}
	}

	// Handle document being edited (status 1) - just log for now
	if data.Status == onlyOfficeStatusDocumentBeingEdited {
		logger.Debugf("OnlyOffice callback: document being edited, key=%s, users=%v", data.Key, data.Users)

		// Send log event for document being edited
		if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
			SendOnlyOfficeLogEvent(logContext, "DEBUG", "callback", fmt.Sprintf("Document being edited, users: %v", data.Users))
		}

		// Handle actions if present
		for _, action := range data.Actions {
			actionMsg := ""
			switch action.Type {
			case 0: // User disconnects
				actionMsg = fmt.Sprintf("User ID %s disconnected from document", action.UserID)
				logger.Debugf("OnlyOffice callback: user ID %s disconnected from document", action.UserID)
			case 1: // New user connects
				actionMsg = fmt.Sprintf("User ID %s connected to document", action.UserID)
				logger.Debugf("OnlyOffice callback: user ID %s connected to document", action.UserID)
			case 2: // User clicked forcesave button
				actionMsg = fmt.Sprintf("User ID %s clicked forcesave button", action.UserID)
				logger.Debugf("OnlyOffice callback: user ID %s clicked forcesave button", action.UserID)
			default:
				actionMsg = fmt.Sprintf("Unknown action type %d for user ID %s", action.Type, action.UserID)
				logger.Debugf("OnlyOffice callback: unknown action type %d for user ID %s", action.Type, action.UserID)
			}

			// Send log event for action
			if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
				SendOnlyOfficeLogEvent(logContext, "DEBUG", "callback", actionMsg)
			}
		}
	}

	// Handle document save operations (status 2, 3, 6, 7)
	if data.Status == onlyOfficeStatusDocumentClosedWithChanges ||
		data.Status == onlyOfficeStatusDocumentSavingError ||
		data.Status == onlyOfficeStatusForceSaveWhileDocumentStillOpen ||
		data.Status == onlyOfficeStatusForceSaveError {

		// Log the save operation details
		statusDesc := ""
		switch data.Status {
		case onlyOfficeStatusDocumentClosedWithChanges:
			statusDesc = "document closed with changes"
		case onlyOfficeStatusDocumentSavingError:
			statusDesc = "document saving error"
		case onlyOfficeStatusForceSaveWhileDocumentStillOpen:
			statusDesc = "force save while document still open"
		case onlyOfficeStatusForceSaveError:
			statusDesc = "force save error"
		}

		logger.Debugf("OnlyOffice callback: processing save operation - %s, key=%s, url=%s, forcesavetype=%d",
			statusDesc, data.Key, data.URL, data.ForceSaveType)

		// Send log event for save operation
		if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
			SendOnlyOfficeLogEvent(logContext, "INFO", "callback", fmt.Sprintf("Processing save operation: %s", statusDesc))
		}

		// Handle history and changes URL if present
		if data.History != nil {
			logger.Debugf("OnlyOffice callback: received history data with serverVersion=%s", data.History.ServerVersion)
			if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
				SendOnlyOfficeLogEvent(logContext, "DEBUG", "callback", fmt.Sprintf("Received history data with serverVersion=%s", data.History.ServerVersion))
			}
		}
		if data.ChangesURL != "" {
			logger.Debugf("OnlyOffice callback: received changes URL: %s", data.ChangesURL)
			if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
				SendOnlyOfficeLogEvent(logContext, "DEBUG", "callback", "Received changes URL for document history")
			}
		}

		// For status 3 (saving error), don't attempt to save the file
		if data.Status == onlyOfficeStatusDocumentSavingError {
			logger.Warningf("OnlyOffice callback: document saving error occurred, not attempting to save")
			return returnOnlyOfficeSuccess(w, r)
		}

		// Check modify permission for save operations
		filePerms, permErr := effectiveFilePerms(d, source)
		if permErr != nil || !filePerms.Modify {
			logger.Errorf("OnlyOffice callback: user %s lacks modify permissions for source=%s path=%s",
				user.Username, source, path)
			return returnOnlyOfficeError(w, r, 403, "user lacks modify permissions")
		}

		downloadURL := resolveOnlyOfficeDownloadURL(data.URL)
		if downloadURL == "" {
			logger.Errorf("OnlyOffice callback: missing or untrusted document URL in callback payload")
			return returnOnlyOfficeError(w, r, 400, "missing or untrusted document URL")
		}

		doc, err := onlyOfficeDownloadClient.Get(downloadURL)
		if err != nil {
			logger.Errorf("OnlyOffice callback: failed to download updated document: %v", err)
			return returnOnlyOfficeError(w, r, 500, "failed to download updated document")
		}
		defer doc.Body.Close()

		// Check if the download was successful
		if doc.StatusCode != 200 {
			logger.Errorf("OnlyOffice callback: failed to download document, status code: %d", doc.StatusCode)
			return returnOnlyOfficeError(w, r, 500, "failed to download document from OnlyOffice server")
		}

		logger.Debugf("OnlyOffice callback: saving document to path=%s",
			path)

		// Send detailed log event for file saving with path information
		if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
			SendOnlyOfficeLogEvent(logContext, "INFO", "callback", fmt.Sprintf("Saving document to path: %s", path))
		}

		// CRITICAL: Validate that the original file still exists before saving
		// This prevents creating duplicate files if the original was renamed/moved
		_, err = files.FileInfoFaster(utils.FileOptions{
			Source: source,
			Path:   path,
		}, user)
		if err != nil {
			logger.Errorf("OnlyOffice callback: original file no longer exists at path=%s: %v",
				path, err)

			// Send error log event with path information
			if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
				SendOnlyOfficeLogEvent(logContext, "ERROR", "callback",
					fmt.Sprintf("Original file no longer exists at path: %s - %v -- was it renamed or moved?", path, err))
			}

			return returnOnlyOfficeError(w, r, 404, "original file no longer exists - it may have been renamed or moved")
		}

		// Get user scope to resolve full index path for write operation
		userScope, err := user.GetScopeForSourceName(source)
		if err != nil {
			return returnOnlyOfficeError(w, r, 403, "user scope not found")
		}
		fullIndexPath := utils.JoinPathAsUnix(userScope, path)

		writeErr := files.WriteFile(source, fullIndexPath, doc.Body)
		if writeErr != nil {
			logger.Errorf("OnlyOffice callback: failed to write updated document to path=%s: %v",
				path, writeErr)

			// Send error log event with path information
			if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
				SendOnlyOfficeLogEvent(logContext, "ERROR", "callback", fmt.Sprintf("Failed to save document to path: %s - %v", path, writeErr))
			}

			return returnOnlyOfficeError(w, r, 500, "failed to save document")
		}

		logger.Infof("OnlyOffice callback: successfully saved document to path=%s",
			path)

		// Send success log event with detailed path information
		if logContext := GetOnlyOfficeLogContext(data.Key); logContext != nil {
			SendOnlyOfficeLogEvent(logContext, "INFO", "callback", fmt.Sprintf("Document saved successfully to path: %s", path))
		}
	}

	// Return success response to OnlyOffice server
	return returnOnlyOfficeSuccess(w, r)
}

// onlyofficeCallbackHandler handles OnlyOffice document server callbacks
//
// @Summary Handle OnlyOffice document server callback
// @Description Receives callbacks from OnlyOffice document server for document status changes and saves
// @Tags Office
// @Accept json
// @Produce json
// @Param source query string false "Source name"
// @Param path query string false "File path"
// @Param hash query string false "Share hash (for public shares)"
// @Success 200 {object} map[string]interface{} "Callback processed successfully"
// @Failure 400 {object} map[string]string "Invalid callback data"
// @Failure 500 {object} map[string]string "Server error"
// @Router /api/office/callback [post]
// @Router /api/office/callback [get]
// @Security ApiKeyAuth
func onlyofficeCallbackHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
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
		return returnOnlyOfficeError(w, r, 405, fmt.Sprintf("unsupported method: %s", r.Method))
	}

	if err != nil {
		logger.Errorf("OnlyOffice callback: failed to parse callback data: %v", err)
		return returnOnlyOfficeError(w, r, 400, "failed to parse callback data")
	}

	if callbackData == nil {
		logger.Errorf("OnlyOffice callback: parsed callback data is nil")
		return returnOnlyOfficeError(w, r, 400, "parsed callback data is nil")
	}

	// Process the callback data using shared logic
	return processOnlyOfficeCallback(w, r, d, callbackData)
}

// parseOnlyOfficeCallbackFromJWT extracts callback data from JWT in Authorization header
func parseOnlyOfficeCallbackFromJWT(r *http.Request) (*OnlyOfficeCallback, error) {
	jwtToken := onlyOfficeCallbackBearerToken(r.Header.Get("Authorization"))
	if jwtToken == "" {
		return nil, errors.New("missing OnlyOffice callback JWT in Authorization header")
	}
	return parseOnlyOfficeCallbackToken(jwtToken)
}

// parseOnlyOfficeCallbackFromJSON extracts callback data from JSON request body
func parseOnlyOfficeCallbackFromJSON(r *http.Request) (*OnlyOfficeCallback, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %v", err)
	}
	if len(body) == 0 {
		return nil, errors.New("empty callback body")
	}

	secret := settings.Config.Integrations.OnlyOffice.Secret
	if secret != "" {
		var wrapper struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Token != "" {
			return parseOnlyOfficeCallbackToken(wrapper.Token)
		}
		if headerToken := onlyOfficeCallbackBearerToken(r.Header.Get("Authorization")); headerToken != "" {
			return parseOnlyOfficeCallbackToken(headerToken)
		}
		return nil, errors.New("unsigned callback rejected when OnlyOffice JWT secret is configured")
	}

	var data OnlyOfficeCallback
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}
	if data.Key == "" {
		return nil, errors.New("missing document key in callback JSON")
	}
	return &data, nil
}

func onlyOfficeCallbackBearerToken(authHeader string) string {
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
}

// parseOnlyOfficeCallbackToken verifies (when secret is configured) and decodes an OnlyOffice callback JWT.
func parseOnlyOfficeCallbackToken(tokenString string) (*OnlyOfficeCallback, error) {
	secret := settings.Config.Integrations.OnlyOffice.Secret
	var claims jwt.MapClaims

	if secret != "" {
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			if t.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return []byte(secret), nil
		})
		if err != nil {
			return nil, fmt.Errorf("invalid OnlyOffice callback JWT: %w", err)
		}
		if !token.Valid {
			return nil, errors.New("invalid OnlyOffice callback JWT: token is not valid")
		}
		mapClaims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, errors.New("invalid OnlyOffice callback JWT claims")
		}
		claims = mapClaims
	} else {
		parser := jwt.NewParser()
		token, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			return nil, fmt.Errorf("failed to parse OnlyOffice callback JWT: %w", err)
		}
		mapClaims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, errors.New("invalid OnlyOffice callback JWT claims")
		}
		claims = mapClaims
	}

	callback, err := onlyOfficeCallbackFromClaims(claims)
	if err != nil {
		return nil, err
	}
	if settings.Config.Integrations.OnlyOffice.Secret != "" && callback.Status == 0 {
		return nil, errors.New("missing callback status in OnlyOffice JWT payload")
	}
	return callback, nil
}

func onlyOfficeCallbackFromClaims(claims jwt.MapClaims) (*OnlyOfficeCallback, error) {
	payload, ok := claims["payload"].(map[string]interface{})
	if !ok {
		payload = map[string]interface{}(claims)
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode callback payload: %w", err)
	}
	var callback OnlyOfficeCallback
	if err := json.Unmarshal(payloadBytes, &callback); err != nil {
		return nil, fmt.Errorf("failed to decode callback payload: %w", err)
	}
	if callback.Key == "" {
		return nil, errors.New("missing document key in callback payload")
	}
	return &callback, nil
}

// validateOnlyOfficeCallbackKey ensures the callback document key matches the active editor session.
// Fails closed when the path cannot be resolved or no editor session key is cached for that file.
func validateOnlyOfficeCallbackKey(source, path string, user *users.User, data *OnlyOfficeCallback) error {
	if data.Key == "" {
		return errors.New("missing document key in callback")
	}
	fi, err := files.FileInfoFaster(utils.FileOptions{
		Path:           path,
		Source:         source,
		Expand:         false,
		FollowSymlinks: true,
	}, user)
	if err != nil {
		return fmt.Errorf("could not resolve document for callback: %w", err)
	}
	if fi == nil || fi.RealPath == "" {
		return errors.New("could not resolve document path for callback")
	}
	expectedKey, err := GetOnlyOfficeId(fi.RealPath)
	if err != nil {
		return errors.New("unknown or expired OnlyOffice editor session for document")
	}
	if expectedKey != data.Key {
		return fmt.Errorf("document key mismatch for path %s", path)
	}
	return nil
}

func GetOnlyOfficeId(realpath string) (string, error) {
	// error is intentionally ignored in order treat errors
	// the same as a cache-miss
	cachedDocumentKey, ok := utils.OnlyOfficeCache.Get(realpath)
	if ok {
		return cachedDocumentKey, nil
	}
	return "", fmt.Errorf("document key not found")
}

func deleteOfficeId(source, path string, user *users.User) {
	fi, err := files.FileInfoFaster(utils.FileOptions{
		Path:           path,
		Source:         source,
		Expand:         false,
		FollowSymlinks: true,
	}, user)
	key := path
	if fi != nil && fi.RealPath != "" {
		key = fi.RealPath
	}
	if err != nil {
		logger.Errorf("deleteOfficeId: failed to resolve realpath, source=%s, path=%s: %v", source, path, err)
	}
	utils.OnlyOfficeCache.Delete(key)
}

// returnOnlyOfficeSuccess returns a success response to OnlyOffice server
func returnOnlyOfficeSuccess(w http.ResponseWriter, r *http.Request) (int, error) {
	resp := map[string]int{
		"error": 0,
	}
	return RenderJSON(w, r, resp)
}

// returnOnlyOfficeError returns an error response to OnlyOffice server with proper status code
func returnOnlyOfficeError(w http.ResponseWriter, r *http.Request, statusCode int, message string) (int, error) {
	// OnlyOffice expects specific error codes in the response body
	errorCode := 0
	switch statusCode {
	case 400:
		errorCode = 1 // Bad request
	case 403:
		errorCode = 1 // Forbidden (treated as bad request by OnlyOffice)
	case 404:
		errorCode = 1 // Not found (treated as bad request by OnlyOffice)
	case 500:
		errorCode = 1 // Internal server error (treated as bad request by OnlyOffice)
	default:
		errorCode = 1 // Default to bad request
	}

	resp := map[string]interface{}{
		"error": errorCode,
	}

	// Log the error for debugging
	logger.Errorf("OnlyOffice callback error (HTTP %d): %s", statusCode, message)

	// Set the appropriate HTTP status code
	w.WriteHeader(statusCode)
	return RenderJSON(w, r, resp)
}
