package http

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/state"
	"github.com/gtsteffaniak/go-logger/logger"
)

// shareListHandler returns a list of all share links.
// @Summary List share links
// @Description Returns a list of share links for the current user, or all links if the user is an admin.
// @Tags Shares
// @Accept json
// @Produce json
// @Success 200 {array} share.ShareFrontend "List of share links"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/share/list [get]
func shareListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var (
		shares []*share.Share
		err    error
	)
	if d.user.Permissions.Admin {
		shares, err = shareStore.All()
	} else {
		shares, err = shareStore.FindByUserID(d.user.ID)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	logger.Debugf("api share/list: user=%q admin=%v shares=%d", d.user.Username, d.user.Permissions.Admin, len(shares))
	return renderJSON(w, r, shareStore.PrepForFrontend(d.user, r, shares...))
}

// shareGetsHandler retrieves share links for a specific resource path.
// @Summary Get share links by path
// @Description Retrieves all share links associated with a specific resource path for the current user.
// @Tags Shares
// @Accept json
// @Produce json
// @Param path query string true "Resource path for which to retrieve share links"
// @Param source query string true "Source name for share links"
// @Success 200 {array} share.ShareFrontend "List of share links for the specified path"
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
	idx := indexing.GetIndex(sourceInfo.Name)
	if idx == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source: %s", sourceName)
	}

	logger.Debug("shareGetHandler querying", "sourceName", sourceName, "sourceInfoPath", sourceInfo.Path, "scopePath", scopePath, "userID", d.user.ID)

	s, err := shareStore.Gets(scopePath, sourceInfo.Path, d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error getting share info from server")
	}
	logger.Debug("shareGetHandler result", "sourceName", sourceName, "scopePath", scopePath, "userID", d.user.ID, "count", len(s))
	return renderJSON(w, r, shareStore.PrepForFrontend(d.user, r, s...))
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
// @Router /api/share [delete]
func shareDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	hash := r.URL.Query().Get("hash")

	if hash == "" {
		return http.StatusBadRequest, nil
	}

	thisShare, err := state.GetShare(hash)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("share not found")
	}
	if !thisShare.UserCanEdit(d.user) {
		return http.StatusForbidden, fmt.Errorf("you are not allowed to delete this share")
	}

	err = state.DeleteShare(hash)
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
// @Success 200 {object} share.ShareFrontend "Updated share link"
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

	sanitizedPath, err := utils.SanitizeUserPath(body.Path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %w", err)
	}
	body.Path = sanitizedPath

	// only allow users to update their own shares
	thisShare, err := shareStore.GetByHash(body.Hash)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("share not found")
	}
	if !thisShare.UserCanEdit(d.user) {
		return http.StatusForbidden, fmt.Errorf("you are not allowed to update this share")
	}
	err = state.UpdateSharePath(body.Hash, body.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	updatedShare, err := shareStore.GetByHash(body.Hash)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	prepared := shareStore.PrepForFrontend(d.user, r, updatedShare)
	if len(prepared) == 0 {
		return http.StatusInternalServerError, fmt.Errorf("could not prepare share response")
	}
	return renderJSON(w, r, prepared[0])
}

// sharePostHandler creates a new share link.
// @Summary Create a share link
// @Description Creates a new share link with an optional expiration time and password protection.
// @Tags Shares
// @Accept json
// @Produce json
// @Param body body share.SharePostBody true "Share creation parameters"
// @Success 200 {object} share.ShareFrontend "Created share link"
// @Failure 400 {object} map[string]string "Bad request - failed to decode body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/share [post]
func sharePostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var req share.SharePostBody
	var err error
	if r.Body != nil {
		if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
		}
		defer r.Body.Close()
	}

	if req.Hash != "" {
		_, err = state.GetShare(req.Hash)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid hash provided")
		}
	}

	var expire int64

	if req.Expires != "" {
		var num int
		num, err = strconv.Atoi(req.Expires)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		var add time.Duration
		switch req.Unit {
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

	hash, status, err2 := getSharePasswordHash(req.Password)
	if err2 != nil {
		return status, err2
	}
	stringHash := ""
	var token string
	if len(hash) > 0 {
		payloadBuffer := make([]byte, 24)
		if _, err = rand.Read(payloadBuffer); err != nil {
			return http.StatusInternalServerError, err
		}
		payload := base64.URLEncoding.EncodeToString(payloadBuffer)

		mac := hmac.New(sha256.New, []byte(config.Auth.Key))
		mac.Write([]byte(payload))
		signature := base64.URLEncoding.EncodeToString(mac.Sum(nil))

		token = payload + "." + signature
		stringHash = string(hash)
	}
	if req.Hash != "" {
		err = state.UpdateShare(req.Hash, func(link *share.Share) error {
			shouldResetCounts := link.DownloadsLimit != req.DownloadsLimit ||
				link.PerUserDownloadLimit != req.PerUserDownloadLimit
			link.Expire = expire
			link.PasswordHash = stringHash
			link.Token = token
			preservedPath := link.Path
			preservedSourcePath := link.SourcePath
			link.FrontendShareInfo = req.FrontendShareInfo
			link.Path = preservedPath
			link.SourcePath = preservedSourcePath
			link.UserID = d.user.ID
			if link.ShareType == "upload" && !req.AllowCreate {
				link.AllowCreate = true
			}
			if shouldResetCounts {
				link.ResetDownloadCounts()
			}
			return nil
		})
		if err != nil {
			return http.StatusInternalServerError, err
		}

		updatedShare, err3 := shareStore.GetByHash(req.Hash)
		if err3 != nil {
			return http.StatusInternalServerError, err3
		}
		prepared := shareStore.PrepForFrontend(d.user, r, updatedShare)
		if len(prepared) == 0 {
			return http.StatusInternalServerError, fmt.Errorf("could not prepare share response")
		}
		return renderJSON(w, r, prepared[0])
	}

	source, ok := config.Server.NameToSource[req.SourceName]
	if !ok {
		return http.StatusForbidden, fmt.Errorf("source with name not found: %s", req.SourceName)
	}

	if source.Config.Private {
		return http.StatusForbidden, fmt.Errorf("the target source is private, sharing is not permitted")
	}

	secureHash, err := generateShortUUID()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	idx := indexing.GetIndex(source.Name)
	if idx == nil {
		return http.StatusForbidden, fmt.Errorf("source with name not found: %s", req.SourceName)
	}
	userscope, err := d.user.GetScopeForSourceName(source.Name)
	if err != nil {
		return http.StatusForbidden, err
	}
	providedPath := req.Path

	cleanPath, err := utils.SanitizeUserPath(providedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	storedPath := utils.JoinPathAsUnix(userscope, cleanPath)

	if req.ShareType == "upload" && !req.AllowCreate {
		req.AllowCreate = true
	}

	s := &share.Share{
		ShareFrontend: share.ShareFrontend{
			FrontendShareInfo: req.FrontendShareInfo,
			Hash:              secureHash,
			Path:              storedPath,
			SourceName:        source.Name,
			Expire:            expire,
		},
		SourcePath:   source.Path,
		UserID:       d.user.ID,
		PasswordHash: stringHash,
		Token:        token,
		Version:      1,
	}
	s.DownloadURL = ""
	s.ShareURL = ""
	s.FaviconUrl = ""
	s.BannerUrl = ""
	s.FrontendShareInfo.SourceURL = ""
	s.FrontendShareInfo.HasPassword = false
	s.CanEditShare = false

	if err = state.CreateShare(s); err != nil {
		return http.StatusInternalServerError, err
	}

	logger.Debug("Created share", "hash", s.Hash, "sourcePath", s.SourcePath, "path", s.Path, "userID", s.UserID)

	created, err := shareStore.GetByHash(secureHash)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	prepared := shareStore.PrepForFrontend(d.user, r, created)
	if len(prepared) == 0 {
		return http.StatusInternalServerError, fmt.Errorf("could not prepare share response")
	}
	return renderJSON(w, r, prepared[0])
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

	// Create the scope path (file: no trailing slash; matches share cache normalization)
	scopePath := utils.JoinPathAsUnix(userscope, path)

	// Check if an existing share already matches these parameters
	existingShares, err := shareStore.Gets(scopePath, sourceInfo.Path, d.user.ID)
	if err == nil && len(existingShares) > 0 {
		for _, existing := range existingShares {
			if existing.DownloadsLimit == downloadCount &&
				existing.MaxBandwidth == downloadSpeed &&
				existing.QuickDownload &&
				(existing.Expire == 0 || existing.Expire >= expire) { // Existing expires later or never

				response := DirectDownloadResponse{
					Status:      "201",
					Hash:        existing.Hash,
					DownloadURL: share.URLFromRequest(r, existing.Hash, true, existing.Token),
					ShareURL:    share.URLFromRequest(r, existing.Hash, false, existing.Token),
				}
				return renderJSON(w, r, response)
			}
		}
	}

	// No matching existing share found, create a new one
	shareLink := &share.Share{
		ShareFrontend: share.ShareFrontend{
			FrontendShareInfo: share.FrontendShareInfo{
				QuickDownload: true,
			},
			MaxBandwidth:   downloadSpeed,
			DownloadsLimit: downloadCount,
			Hash:           secureHash,
			Path:           scopePath,
			SourceName:     sourceInfo.Name,
			Expire:         expire,
		},
		SourcePath: idx.Path,
		UserID:     d.user.ID,
		Version:    1,
	}

	if err = state.CreateShare(shareLink); err != nil {
		return http.StatusInternalServerError, err
	}

	snap, err := state.GetShare(secureHash)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	response := DirectDownloadResponse{
		Status:      "200",
		Hash:        secureHash,
		DownloadURL: share.URLFromRequest(r, secureHash, true, snap.Token),
		ShareURL:    share.URLFromRequest(r, secureHash, false, snap.Token),
	}

	return renderJSON(w, r, response)
}

// shareInfoHandler retrieves share information by hash.
// @Summary Get share information by hash
// @Description Returns information about a share link based on its hash. This endpoint is publicly accessible and can be used with or without authentication.
// @Tags Shares
// @Accept json
// @Produce json
// @Param hash query string true "Hash of the share link"
// @Success 200 {object} share.Share "Share information"
// @Failure 404 {object} map[string]string "Share hash not found"
// @Router /public/api/share/info [get]
func shareInfoHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	hash := r.URL.Query().Get("hash")
	shareInfo, err := state.GetShare(hash)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("share hash not found")
	}
	frontendShareInfo := shareInfo.FrontendShareInfo
	frontendShareInfo.ShareURL = share.URLFromRequest(r, hash, false, "")
	frontendShareInfo.BannerUrl = shareInfo.BannerURL()
	frontendShareInfo.FaviconUrl = shareInfo.FaviconURL()
	filtered := make([]users.SidebarLink, 0, len(frontendShareInfo.SidebarLinks))
	for _, link := range frontendShareInfo.SidebarLinks {
		if link.Category == "download" && frontendShareInfo.ShareType == "upload" {
			continue
		}
		filtered = append(filtered, link)
	}
	frontendShareInfo.SidebarLinks = filtered
	frontendShareInfo.SourceURL = shareInfo.SourceURL(d.user)
	frontendShareInfo.CanEditShare = shareInfo.UserCanEdit(d.user)
	if frontendShareInfo.SourceURL != "" {
		frontendShareInfo.SidebarLinks = append(frontendShareInfo.SidebarLinks, users.SidebarLink{
			Name:     "sourceLocation",
			Category: "custom",
			Target:   frontendShareInfo.SourceURL,
		})
	}
	return renderJSON(w, r, frontendShareInfo)
}

func getSharePasswordHash(plaintextPassword string) (data []byte, statuscode int, err error) {
	if plaintextPassword == "" {
		return nil, 0, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), bcrypt.DefaultCost)
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

// publicSharePatchPinnedItemsHandler adds or removes a pinned item on a share link.
// @Summary Add or remove a pinned item on a share
// @Description Patches one pinned item at a time for share owners. Defaults to add; pass ?action=remove to unpin.
// @Tags Shares
// @Accept json
// @Produce json
// @Param hash query string true "Share hash"
// @Param action query string false "add (default) or remove"
// @Param body body sharePinnedItemPatchRequest true "Pinned item"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 404 {object} map[string]string "Share not found"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /public/api/share/pinnedItems [patch]
func sharePatchPinnedItemsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		return http.StatusBadRequest, fmt.Errorf("hash is required")
	}

	link, err := state.GetShare(hash)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("share hash not found")
	}

	if d.user.Username == "anonymous" || !link.UserCanEdit(d.user) {
		return http.StatusForbidden, fmt.Errorf("share pin editing is not allowed for this user")
	}

	if link.ShareType == "upload" {
		return http.StatusForbidden, fmt.Errorf("pinning is disabled for upload shares")
	}

	var body sharePinnedItemPatchRequest
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
	}
	defer r.Body.Close()

	action := pinnedItemAction(r)
	if action != "add" && action != "remove" {
		return http.StatusBadRequest, fmt.Errorf("action must be add or remove")
	}

	if body.Path == "" || body.Name == "" {
		return http.StatusBadRequest, fmt.Errorf("path and name are required")
	}

	shareRelDir, err := normalizeShareRelativeDir(body.Path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %s", body.Path)
	}

	cleanName, err := utils.SanitizeUserPath(body.Name)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid name: %s", body.Name)
	}
	body.Name = cleanName

	pinned := link.EnsurePinnedItems()
	switch action {
	case "add":
		pinned.Add(shareRelDir, body.Name)
	case "remove":
		pinned.Remove(shareRelDir, body.Name)
	}

	if err := state.UpdateShare(hash, func(existing *share.Share) error {
		existing.PinnedItems = link.PinnedItems
		return nil
	}); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to save share: %w", err)
	}

	return http.StatusNoContent, nil
}
