package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// ParsedURLInfo holds the parsed URL information
type ParsedURLInfo struct {
	Source        string
	Path          string
	EncodedPath   string
	IsPublicShare bool
	DecodedURL    string
	BaseURL       string
	ShareHash     string
}

// parseOnlyOfficeURL parses and validates the OnlyOffice URL from the request
func parseOnlyOfficeURL(encodedURL string) (*ParsedURLInfo, error) {
	decodedURL, err := url.QueryUnescape(encodedURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL encoding: %v", err)
	}

	info := &ParsedURLInfo{
		DecodedURL: decodedURL,
	}

	if strings.Contains(decodedURL, "public/api/raw") {
		return parsePublicShareURL(info, encodedURL)
	}

	return parseRegularResourceURL(info, encodedURL)
}

// parsePublicShareURL handles public share URL format: /public/api/raw?path=/file.doc&hash=...
func parsePublicShareURL(info *ParsedURLInfo, encodedURL string) (*ParsedURLInfo, error) {
	info.IsPublicShare = true

	// Parse the URL to extract query parameters
	parsedURL, err := url.Parse(info.DecodedURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL format: %v", err)
	}

	info.Path = parsedURL.Query().Get("path")
	info.ShareHash = parsedURL.Query().Get("hash")

	if info.Path == "" || info.ShareHash == "" {
		return nil, fmt.Errorf("missing path or hash parameter in public share URL")
	}

	// Extract base URL
	parts := strings.Split(info.DecodedURL, "/public/api/raw")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid public share URL format")
	}
	info.BaseURL = parts[0]

	return info, nil
}

// parseRegularResourceURL handles regular resource URL format: /api/raw?files=source::path
func parseRegularResourceURL(info *ParsedURLInfo, encodedURL string) (*ParsedURLInfo, error) {
	info.IsPublicShare = false

	// Extract the files parameter
	decodedParts := strings.Split(info.DecodedURL, "/api/raw?files=")
	encodedParts := strings.Split(encodedURL, "/api/raw?files=")

	if len(decodedParts) < 2 || len(encodedParts) < 2 {
		return nil, fmt.Errorf("invalid resource URL format")
	}

	info.EncodedPath = encodedParts[1]
	sourceFile := decodedParts[1]

	// Parse source::path format
	if err := parseSourcePath(sourceFile, info); err != nil {
		return nil, err
	}

	// Extract base URL
	baseURLParts := strings.Split(info.DecodedURL, "/api/raw")
	if len(baseURLParts) < 1 {
		return nil, fmt.Errorf("invalid resource URL format")
	}
	info.BaseURL = baseURLParts[0]

	return info, nil
}

// parseSourcePath parses the source::path format and validates it
func parseSourcePath(sourceFile string, info *ParsedURLInfo) error {
	const delimiter = "::"

	parts := strings.Split(sourceFile, delimiter)
	if len(parts) != 2 {
		return fmt.Errorf("invalid source::path format in URL: %s", sourceFile)
	}

	info.Source = strings.TrimSpace(parts[0])
	info.Path = strings.TrimSpace(parts[1])

	if info.Source == "" || info.Path == "" {
		return fmt.Errorf("source and path cannot be empty")
	}

	return nil
}

// processPublicShare handles the authentication and path construction for public shares
func processPublicShare(info *ParsedURLInfo, r *http.Request) error {
	// Get the share link by hash
	link, err := store.Share.GetByHash(info.ShareHash)
	if err != nil {
		return fmt.Errorf("share not found")
	}

	// Authenticate the share request
	if link.Hash != "" {
		status, err := authenticateShareRequest(r, link)
		if err != nil || status != http.StatusOK {
			return fmt.Errorf("could not authenticate share request")
		}
	}

	// Use the actual source from the share link
	info.Source = link.Source

	// Construct the full path like in withHashFileHelper
	fullPath := constructSharePath(link.Path, info.Path)
	info.Path = fullPath

	// Create encoded path for callback
	info.EncodedPath = info.Source + "::" + url.QueryEscape(info.Path)

	return nil
}

// constructSharePath builds the complete path for shared resources
func constructSharePath(linkPath, requestPath string) string {
	if requestPath == "" || requestPath == "/" {
		return linkPath
	}

	return strings.TrimSuffix(linkPath, "/") + "/" + strings.TrimPrefix(requestPath, "/")
}

// adjustURLForInternalConfig modifies URLs based on internal server configuration
func adjustURLForInternalConfig(info *ParsedURLInfo) {
	if settings.Config.Server.InternalUrl == "" {
		return
	}

	info.BaseURL = settings.Config.Server.InternalUrl

	var replacement string
	if info.IsPublicShare {
		parts := strings.Split(info.DecodedURL, "/public/api/raw")
		if len(parts) > 0 {
			replacement = parts[0]
		}
	} else {
		parts := strings.Split(info.DecodedURL, "/api/raw")
		if len(parts) > 0 {
			replacement = parts[0]
		}
	}

	if replacement != "" {
		info.DecodedURL = strings.Replace(info.DecodedURL, replacement, settings.Config.Server.InternalUrl, 1)
	}
}

func onlyofficeClientConfigGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if settings.Config.Integrations.OnlyOffice.Url == "" {
		return http.StatusInternalServerError, errors.New("only-office integration must be configured in settings")
	}

	encodedURL := r.URL.Query().Get("url")
	if encodedURL == "" {
		return http.StatusBadRequest, errors.New("missing url parameter")
	}

	// Parse the URL into structured information
	urlInfo, err := parseOnlyOfficeURL(encodedURL)
	if err != nil {
		return http.StatusBadRequest, err
	}

	// Handle public share authentication and path construction
	if urlInfo.IsPublicShare {
		if err = processPublicShare(urlInfo, r); err != nil {
			return http.StatusForbidden, err
		}
	}

	// Adjust URLs based on internal server configuration
	adjustURLForInternalConfig(urlInfo)

	// Get file information based on share type
	fileInfo, err := getFileInfo(urlInfo, d)
	if err != nil {
		return errToStatus(err), err
	}

	// Generate OnlyOffice document ID
	id, err := getOnlyOfficeId(urlInfo.Source, fileInfo.Path)
	if err != nil {
		return http.StatusNotFound, err
	}

	// Build and return client configuration
	clientConfig, err := buildClientConfig(urlInfo, fileInfo, id, d)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, clientConfig)
}

// getFileInfo retrieves file information based on whether it's a public share or regular resource
func getFileInfo(urlInfo *ParsedURLInfo, d *requestContext) (iteminfo.ExtendedFileInfo, error) {
	if urlInfo.IsPublicShare {
		// Get source configuration from the source map
		sourceConfig, ok := config.Server.SourceMap[urlInfo.Source]
		if !ok {
			return iteminfo.ExtendedFileInfo{}, fmt.Errorf("source not found")
		}

		return files.FileInfoFaster(iteminfo.FileOptions{
			Access:   store.Access,
			Username: "public", // Use public user for shares
			Path:     urlInfo.Path,
			Modify:   false, // Public shares don't allow modification
			Source:   sourceConfig.Name,
			Expand:   false,
		})
	}

	// Handle regular authenticated resource
	userScope, err := settings.GetScopeFromSourceName(d.user.Scopes, urlInfo.Source)
	if err != nil {
		return iteminfo.ExtendedFileInfo{}, err
	}

	return files.FileInfoFaster(iteminfo.FileOptions{
		Access:   store.Access,
		Username: d.user.Username,
		Path:     utils.JoinPathAsUnix(userScope, urlInfo.Path),
		Modify:   d.user.Permissions.Modify,
		Source:   urlInfo.Source,
		Expand:   false,
	})
}

// buildClientConfig constructs the OnlyOffice client configuration
func buildClientConfig(urlInfo *ParsedURLInfo, fileInfo iteminfo.ExtendedFileInfo, id string, d *requestContext) (map[string]interface{}, error) {
	// Extract file type
	fileType := extractFileType(fileInfo.Name)

	// Determine theme
	theme := "light"
	if d.user.DarkMode {
		theme = "dark"
	}

	// Determine editing permissions
	canEdit := determineEditPermissions(urlInfo.IsPublicShare, d.user.Permissions.Modify, fileType)

	mode := "view"
	if canEdit {
		mode = "edit"
	}

	// Build callback URL
	callbackURL := fmt.Sprintf("%v/api/onlyoffice/callback?path=%v&auth=%v",
		urlInfo.BaseURL, urlInfo.EncodedPath, d.token)

	// Build client configuration
	clientConfig := map[string]interface{}{
		"document": map[string]interface{}{
			"fileType": fileType,
			"key":      id,
			"title":    fileInfo.Name,
			"url":      urlInfo.DecodedURL + "&auth=" + d.token,
			"permissions": map[string]interface{}{
				"edit":     canEdit,
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
				"uiTheme":   theme,
			},
			"lang": d.user.Locale,
			"mode": mode,
		},
	}

	// Add JWT token if secret is configured
	if settings.Config.Integrations.OnlyOffice.Secret != "" {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(clientConfig))
		signature, err := token.SignedString([]byte(settings.Config.Integrations.OnlyOffice.Secret))
		if err != nil {
			return nil, fmt.Errorf("failed to sign JWT")
		}
		clientConfig["token"] = signature
	}

	return clientConfig, nil
}

// extractFileType safely extracts the file extension
func extractFileType(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}

// determineEditPermissions determines if the user can edit the document
func determineEditPermissions(isPublicShare bool, userCanModify bool, fileType string) bool {
	if isPublicShare {
		return false // Public shares typically don't allow editing
	}
	return iteminfo.CanEditOnlyOffice(userCanModify, fileType)
}

// parseCallbackPath parses the encoded path from callback requests
func parseCallbackPath(encodedPath string) (source, path string, err error) {
	const delimiter = "::"

	parts := strings.Split(encodedPath, delimiter)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid callback path format: expected source::path")
	}

	source = strings.TrimSpace(parts[0])
	if source == "" {
		return "", "", fmt.Errorf("source cannot be empty")
	}

	path, err = url.QueryUnescape(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid path encoding: %v", err)
	}

	return source, path, nil
}

func onlyofficeCallbackHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	var data OnlyOfficeCallback
	err = json.Unmarshal(body, &data)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	encodedPath := r.URL.Query().Get("path")
	if encodedPath == "" {
		return http.StatusBadRequest, fmt.Errorf("missing path parameter")
	}

	source, path, err := parseCallbackPath(encodedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}
	if data.Status == onlyOfficeStatusDocumentClosedWithChanges ||
		data.Status == onlyOfficeStatusDocumentClosedWithNoChanges {
		// Refer to only-office documentation
		// - https://api.onlyoffice.com/editors/coedit
		// - https://api.onlyoffice.com/editors/callback
		//
		// When the document is fully closed by all editors,
		// then the document key should no longer be re-used.
		deleteOfficeId(source, path)
	}

	if data.Status == onlyOfficeStatusDocumentClosedWithChanges ||
		data.Status == onlyOfficeStatusForceSaveWhileDocumentStillOpen {

		// Public shares typically don't allow modification
		if source == "public" {
			return http.StatusForbidden, fmt.Errorf("public shares do not allow modifications")
		}

		if !d.user.Permissions.Modify {
			return http.StatusForbidden, nil
		}

		doc, err := http.Get(data.URL)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		defer doc.Body.Close()

		fileOpts := iteminfo.FileOptions{
			Path:   path,
			Source: source,
		}
		writeErr := files.WriteFile(fileOpts, doc.Body)
		if writeErr != nil {
			return http.StatusInternalServerError, writeErr
		}
	}

	resp := map[string]int{
		"error": 0,
	}
	return renderJSON(w, r, resp)
}

func getOnlyOfficeId(source, path string) (string, error) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return "", fmt.Errorf("source not found")
	}
	realpath, _, _ := idx.GetRealPath(path)
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

func onlyofficeGetTokenHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// get config from body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer r.Body.Close()

	var payload map[string]interface{}
	// marshall to struct
	err = json.Unmarshal(body, &payload)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if settings.Config.Integrations.OnlyOffice.Secret != "" {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims(payload))
		ss, err := token.SignedString([]byte(settings.Config.Integrations.OnlyOffice.Secret))
		if err != nil {
			return 500, errors.New("could not generate a new jwt")
		}
		return renderJSON(w, r, map[string]string{"token": ss})
	}
	return 400, fmt.Errorf("bad request")
}
