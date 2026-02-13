package http

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
)

// ShareResponse represents a share with computed username field and download URL
type ShareResponse struct {
	*share.Link
	Source     string `json:"source"` // Override embedded field to show source name
	Username   string `json:"username,omitempty"`
	PathExists bool   `json:"pathExists"`
}

// convertToFrontendShareResponse converts shares to response format with usernames
func convertToFrontendShareResponse(r *http.Request, shares []*share.Link) ([]*ShareResponse, error) {
	responses := make([]*ShareResponse, 0, len(shares))
	for _, s := range shares {
		user, err := store.Users.Get(s.UserID)
		username := ""
		if err == nil {
			username = user.Username
		}

		// Get source info to convert path to name for frontend
		sourceInfo, ok := config.Server.SourceMap[s.Source]
		if !ok {
			// Source not found - likely corrupted data. Try to find by name as fallback
			sourceInfo, ok = config.Server.NameToSource[s.Source]
			if !ok {
				continue
			}
			// Found by name - this is corrupted data, fix it
			logger.Warning("Share has corrupted source - fixing", "hash", s.Hash, "from", s.Source, "to", sourceInfo.Path)
			s.Source = sourceInfo.Path
			_ = store.Share.Save(s) // Best effort fix
		}

		// Check if the path exists on the filesystem
		pathExists := utils.CheckPathExists(filepath.Join(sourceInfo.Path, s.Path))

		s.CommonShare.HasPassword = s.HasPassword()
		s.DownloadURL = getShareURL(r, s.Hash, true, s.Token)
		s.ShareURL = getShareURL(r, s.Hash, false, s.Token)
		// Create response with source name (overrides the embedded Link's source field)
		responses = append(responses, &ShareResponse{
			Link:       s,
			Source:     sourceInfo.Name, // Override to show source name instead of backend path
			Username:   username,
			PathExists: pathExists,
		})
	}
	return responses, nil
}

// shareListHandler returns a list of all share links.
// @Summary List share links
// @Description Returns a list of share links for the current user, or all links if the user is an admin.
// @Tags Shares
// @Accept json
// @Produce json
// @Success 200 {array} share.Link "List of share links"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/shares [get]
func shareListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var err error
	var shares []*share.Link
	if d.user.Permissions.Admin {
		shares, err = store.Share.All()
	} else {
		shares, err = store.Share.FindByUserID(d.user.ID)
	}
	if err != nil && err != errors.ErrNotExist {
		return http.StatusInternalServerError, err
	}
	shares = utils.NonNilSlice(shares)
	sharesWithUsernames, err := convertToFrontendShareResponse(r, shares)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, sharesWithUsernames)
}

// shareGetsHandler retrieves share links for a specific resource path.
// @Summary Get share links by path
// @Description Retrieves all share links associated with a specific resource path for the current user.
// @Tags Shares
// @Accept json
// @Produce json
// @Param path query string true "Resource path for which to retrieve share links"
// @Param source query string true "Source name for share links"
// @Success 200 {array} share.Link "List of share links for the specified path"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/share [get]
func shareGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	sourceName := r.URL.Query().Get("source")
	sourceInfo, ok := config.Server.NameToSource[sourceName] // backend source is path
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source name: %s", sourceName)
	}
	userscope, err := d.user.GetScopeForSourceName(sourceName)
	if err != nil {
		return http.StatusForbidden, err
	}
	scopePath := utils.JoinPathAsUnix(userscope, path)
	scopePath = utils.AddTrailingSlashIfNotExists(scopePath)
	s, err := store.Share.Gets(scopePath, sourceInfo.Path, d.user.ID)
	if err == errors.ErrNotExist || len(s) == 0 {
		return renderJSON(w, r, []*ShareResponse{})
	}
	// DownloadURL will be set in convertToFrontendShareResponse

	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error getting share info from server")
	}
	sharesWithUsernames, err := convertToFrontendShareResponse(r, s)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, sharesWithUsernames)
}

// shareDeleteHandler deletes a specific share link by its hash.
// @Summary Delete a share link
// @Description Deletes a share link specified by its hash.
// @Tags Shares
// @Accept json
// @Produce json
// @Param hash query string true "Hash of the share link to delete"
// @Success 200 "Share link deleted successfully"
// @Failure 400 {object} map[string]string "Bad request - missing or invalid hash"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/shares [delete]
func shareDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	hash := r.URL.Query().Get("hash")

	if hash == "" {
		return http.StatusBadRequest, nil
	}

	err := store.Share.Delete(hash)
	if err != nil {
		return errToStatus(err), err
	}

	return errToStatus(err), err
}

// sharePatchHandler updates a share link's path.
// @Summary Update share link path
// @Description Updates the path for a specific share link identified by hash
// @Tags Shares
// @Accept json
// @Produce json
// @Param body body object{hash=string,path=string} true "Hash and new path"
// @Success 200 {object} ShareResponse "Updated share link"
// @Failure 400 {object} map[string]string "Bad request - missing or invalid parameters"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/share [patch]
func sharePatchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var body struct {
		Hash string `json:"hash"`
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
	}
	defer r.Body.Close()

	if body.Hash == "" || body.Path == "" {
		return http.StatusBadRequest, fmt.Errorf("hash and path are required")
	}

	// Update the share path
	err := store.Share.UpdateSharePath(body.Hash, body.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Get the updated share
	updatedShare, err := store.Share.GetByHash(body.Hash)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Convert to response format
	sharesWithUsernames, err := convertToFrontendShareResponse(r, []*share.Link{updatedShare})
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, sharesWithUsernames[0])
}

// sharePostHandler creates a new share link.
// @Summary Create a share link
// @Description Creates a new share link with an optional expiration time and password protection.
// @Tags Shares
// @Accept json
// @Produce json
// @Param body body share.CreateBody true "Share creation parameters"
// @Success 200 {object} share.Link "Created share link"
// @Failure 400 {object} map[string]string "Bad request - failed to decode body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/share [post]
func sharePostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var s *share.Link
	var err error
	var body share.CreateBody
	if r.Body != nil {
		if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
		}
		defer r.Body.Close()
	}

	// check if body.Hash is a valid hash
	if body.Hash != "" {
		s, err = store.Share.GetByHash(body.Hash)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid hash provided")
		}
	}

	var expire int64 = 0

	if body.Expires != "" {
		var num int
		num, err = strconv.Atoi(body.Expires)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		var add time.Duration
		switch body.Unit {
		case "seconds":
			add = time.Second * time.Duration(num)
		case "minutes":
			add = time.Minute * time.Duration(num)
		case "days":
			add = time.Hour * 24 * time.Duration(num)
		default:
			add = time.Hour * time.Duration(num)
		}

		expire = time.Now().Add(add).Unix()
	}

	hash, status, err := getSharePasswordHash(body)
	if err != nil {
		return status, err
	}
	stringHash := ""
	var token string
	if len(hash) > 0 {
		// Generate a cryptographically secure token similar to JWT
		// Create a random payload
		payloadBuffer := make([]byte, 24)
		if _, err = rand.Read(payloadBuffer); err != nil {
			return http.StatusInternalServerError, err
		}
		payload := base64.URLEncoding.EncodeToString(payloadBuffer)

		// Sign the payload with HMAC-SHA256 using the same secret key as JWT tokens
		mac := hmac.New(sha256.New, []byte(config.Auth.Key))
		mac.Write([]byte(payload))
		signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

		// Combine payload and signature: payload.signature (similar to JWT format)
		token = payload + "." + signature
		stringHash = string(hash)
	}
	if s != nil {
		// Check if downloads limit or per-user limit changed - reset counts if so
		shouldResetCounts := s.DownloadsLimit != body.DownloadsLimit || s.PerUserDownloadLimit != body.PerUserDownloadLimit

		s.Expire = expire
		s.PasswordHash = stringHash
		s.Token = token
		// Preserve immutable fields for updates. Path and Source should not change on edits.
		// If the request attempts to provide empty values (or any values) for these,
		// keep the existing ones from the stored share.
		body.Path = s.Path
		body.Source = s.Source
		s.CommonShare = body.CommonShare
		if s.ShareType == "upload" && !body.AllowCreate {
			s.AllowCreate = true
		}

		// Reset download counts if limit settings changed
		if shouldResetCounts {
			s.ResetDownloadCounts()
		}

		if err = store.Share.Save(s); err != nil {
			return http.StatusInternalServerError, err
		}
		// Convert to ShareResponse format with username
		var user *users.User
		user, err = store.Users.Get(s.UserID)
		username := ""
		if err == nil {
			username = user.Username
		}
		response := &ShareResponse{
			Link:     s,
			Username: username,
		}
		return renderJSON(w, r, response)
	}

	source, ok := config.Server.NameToSource[body.Source]
	if !ok {
		return http.StatusForbidden, fmt.Errorf("source with name not found: %s", body.Source)
	}

	if source.Config.Private {
		return http.StatusForbidden, fmt.Errorf("the target source is private, sharing is not permitted")
	}

	// create a new share link
	secure_hash, err := generateShortUUID()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	// validate source path exists
	idx := indexing.GetIndex(source.Name)
	if idx == nil {
		return http.StatusForbidden, fmt.Errorf("source with name not found: %s", body.Source)
	}
	userscope, err := d.user.GetScopeForSourceName(source.Name)
	if err != nil {
		return http.StatusForbidden, err
	}
	providedPath := body.Path

	// Rule 1: Validate user-provided path to prevent path traversal
	cleanPath, err := utils.SanitizeUserPath(providedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %v", err)
	}

	body.Path = utils.JoinPathAsUnix(userscope, cleanPath)
	body.Path = utils.AddTrailingSlashIfNotExists(body.Path)
	// validate path exists as file or folder
	_, _, err = idx.GetRealPath(body.Path)
	if err != nil {
		return http.StatusForbidden, fmt.Errorf("path not found: %s", providedPath)
	}

	if body.ShareType == "upload" && !body.AllowCreate {
		body.AllowCreate = true
	}
	body.Source = source.Path // backend source is path
	s = &share.Link{
		Expire:       expire,
		UserID:       d.user.ID,
		Hash:         secure_hash,
		PasswordHash: stringHash,
		Token:        token,
		CommonShare:  body.CommonShare,
		Version:      1, // Set version for new shares
	}
	if err = store.Share.Save(s); err != nil {
		return http.StatusInternalServerError, err
	}
	sharesWithUsernames, err := convertToFrontendShareResponse(r, []*share.Link{s})
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, sharesWithUsernames[0])
}

// DirectDownloadResponse represents the response for direct download endpoint
type DirectDownloadResponse struct {
	Status      string `json:"status"`
	Hash        string `json:"hash"`
	DownloadURL string `json:"url"`
	ShareURL    string `json:"shareUrl"`
}

// shareDirectDownloadHandler creates a direct download link for files only.
// @Summary Create direct download link
// @Description Creates a direct download link for a specific file with configurable duration, download count, and speed limits. If a share already exists with matching parameters, the existing share will be reused.
// @Tags Shares
// @Accept json
// @Produce json
// @Param path query string true "File path to create download link for"
// @Param source query string true "Source name for the file"
// @Param duration query string false "Duration in minutes for link validity (default: 60)"
// @Param count query string false "Maximum number of downloads allowed (default: unlimited)"
// @Param speed query string false "Download speed limit in kbps (default: unlimited)"
// @Success 201 {object} DirectDownloadResponse "Direct download link created"
// @Failure 400 {object} map[string]string "Bad request - invalid parameters or path is not a file"
// @Failure 403 {object} map[string]string "Forbidden - access denied"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/share/direct [get]
func shareDirectDownloadHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Extract query parameters
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	duration := r.URL.Query().Get("duration")
	downloadCountStr := r.URL.Query().Get("count")
	downloadSpeedStr := r.URL.Query().Get("speed")

	// Validate required parameters
	if path == "" || source == "" {
		return http.StatusBadRequest, fmt.Errorf("path and source are required")
	}

	// Validate source exists
	sourceInfo, ok := config.Server.NameToSource[source]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source name: %s", source)
	}

	// Get user scope for this source
	userscope, err := d.user.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}

	// Validate the path exists and is a file (not a folder)
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusForbidden, fmt.Errorf("source with name not found: %s", source)
	}

	metadata, exists := idx.GetReducedMetadata(path, false)
	if !exists {
		return http.StatusBadRequest, fmt.Errorf("path is either not a file or not found: %s", path)
	}

	// Check if it's a file (not a directory)
	if metadata.Type == "directory" {
		return http.StatusBadRequest, fmt.Errorf("path must be a file, not a directory: %s", path)
	}

	// Set default duration to 60 minutes if not provided
	if duration == "" {
		duration = "60"
	}

	// Parse download count
	var downloadCount int
	if downloadCountStr != "" {
		downloadCount, err = strconv.Atoi(downloadCountStr)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid downloadCount: %v", err)
		}
	}

	// Parse download speed (in bytes per second)
	var downloadSpeed int
	if downloadSpeedStr != "" {
		downloadSpeed, err = strconv.Atoi(downloadSpeedStr)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid downloadSpeed: %v", err)
		}
	}

	// Calculate expiration time
	durationNum, err := strconv.Atoi(duration)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid duration: %v", err)
	}
	expire := time.Now().Add(time.Minute * time.Duration(durationNum)).Unix()

	// Generate secure hash for the share
	secureHash, err := generateShortUUID()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// Create the scope path
	scopePath := utils.JoinPathAsUnix(userscope, path)

	// Check if an existing share already matches these parameters
	existingShares, err := store.Share.Gets(scopePath, sourceInfo.Path, d.user.ID)
	if err == nil && len(existingShares) > 0 {
		// Look for a share that matches our parameters
		for _, existing := range existingShares {
			if existing.DownloadsLimit == downloadCount &&
				existing.MaxBandwidth == downloadSpeed &&
				existing.QuickDownload &&
				(existing.Expire == 0 || existing.Expire >= expire) { // Existing expires later or never

				response := DirectDownloadResponse{
					Status:      "201",
					Hash:        existing.Hash,
					DownloadURL: getShareURL(r, existing.Hash, true, existing.Token),
					ShareURL:    getShareURL(r, existing.Hash, false, existing.Token),
				}
				return renderJSON(w, r, response)
			}
		}
	}

	// No matching existing share found, create a new one
	shareLink := &share.Link{
		Expire:  expire,
		UserID:  d.user.ID,
		Hash:    secureHash,
		Version: 1, // Set version for new shares
		CommonShare: share.CommonShare{
			Path:           scopePath,
			Source:         idx.Path,
			DownloadsLimit: downloadCount,
			MaxBandwidth:   downloadSpeed,
			QuickDownload:  true, // Enable quick download for direct downloads
		},
	}

	// Save the share
	if err := store.Share.Save(shareLink); err != nil {
		return http.StatusInternalServerError, err
	}

	// Return response
	response := DirectDownloadResponse{
		Status:      "200",
		Hash:        secureHash,
		DownloadURL: getShareURL(r, secureHash, true, shareLink.Token),
		ShareURL:    getShareURL(r, secureHash, false, shareLink.Token),
	}

	return renderJSON(w, r, response)
}

func getShareURL(r *http.Request, hash string, isDirectDownload bool, token string) string {
	var shareURL string
	tokenParam := ""
	if token != "" && isDirectDownload {
		tokenParam = fmt.Sprintf("&token=%s", url.QueryEscape(token))
	}

	if config.Server.ExternalUrl != "" {
		if isDirectDownload {
			shareURL = fmt.Sprintf("%s%spublic/api/raw?hash=%s%s", config.Server.ExternalUrl, config.Server.BaseURL, hash, tokenParam)
		} else {
			shareURL = fmt.Sprintf("%s%spublic/share/%s", config.Server.ExternalUrl, config.Server.BaseURL, hash)
		}

	} else {
		// Prefer X-Forwarded-Host for proxy support
		var host string
		var scheme string
		if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
			// Use X-Forwarded-Proto if available, otherwise default to https for proxied requests
			if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
				scheme = forwardedProto
			} else {
				scheme = "https"
			}
		} else {
			// Fallback to simple approach
			host = r.Host
			scheme = getScheme(r)
		}
		if isDirectDownload {
			shareURL = fmt.Sprintf("%s://%s%spublic/api/raw?hash=%s%s", scheme, host, config.Server.BaseURL, hash, tokenParam)
		} else {
			shareURL = fmt.Sprintf("%s://%s%spublic/share/%s", scheme, host, config.Server.BaseURL, hash)
		}
	}
	return shareURL
}

// shareInfoHandler retrieves share information by hash.
// @Summary Get share information by hash
// @Description Returns information about a share link based on its hash. This endpoint is publicly accessible and can be used with or without authentication.
// @Tags Shares
// @Accept json
// @Produce json
// @Param hash query string true "Hash of the share link"
// @Success 200 {object} share.CommonShare "Share information"
// @Failure 404 {object} map[string]string "Share hash not found"
// @Router /public/api/shareinfo [get]
func shareInfoHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	hash := r.URL.Query().Get("hash")
	// Get the file link by hash (need full Link to get Token)
	shareLink, err := store.Share.GetByHash(hash)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("share hash not found")
	}
	commonShare := shareLink.CommonShare
	commonShare.DownloadURL = getShareURL(r, hash, true, shareLink.Token)
	commonShare.ShareURL = getShareURL(r, hash, false, shareLink.Token)
	_, _, err = getShareImagePartsHelper(shareLink, true)
	if err == nil {
		commonShare.BannerUrl = fmt.Sprintf("%spublic/api/share/image?banner=true&hash=%s", config.Server.BaseURL, hash)
	}
	_, _, err = getShareImagePartsHelper(shareLink, false)
	if err == nil {
		commonShare.FaviconUrl = fmt.Sprintf("%spublic/api/share/image?favicon=true&hash=%s", config.Server.BaseURL, hash)
	}
	commonShare.Source = ""
	commonShare.Path = ""
	commonShare.SidebarLinks = []users.SidebarLink{}
	for _, link := range shareLink.SidebarLinks {
		if link.Category == "download" && shareLink.ShareType == "upload" {
			continue
		} else {
			commonShare.SidebarLinks = append(commonShare.SidebarLinks, link)
		}
	}
	return renderJSON(w, r, commonShare)
}

func getSharePasswordHash(body share.CreateBody) (data []byte, statuscode int, err error) {
	if body.Password == "" {
		return nil, 0, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to hash password")
	}

	return hash, 0, nil
}

func generateShortUUID() (string, error) {
	// Generate 16 random bytes (128 bits of entropy)
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Encode the bytes to a URL-safe base64 string
	uuid := base64.RawURLEncoding.EncodeToString(bytes)

	// Trim the length to 22 characters for a shorter ID
	return uuid[:22], nil
}

func redirectToShare(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Remove the base URL and "/share/" prefix to get the full path after share
	sharePath := strings.TrimPrefix(r.URL.Path, config.Server.BaseURL+"share/")
	newURL := config.Server.BaseURL + "public/share/" + sharePath
	if r.URL.RawQuery != "" {
		newURL += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, newURL, http.StatusMovedPermanently)
	return http.StatusMovedPermanently, nil
}
