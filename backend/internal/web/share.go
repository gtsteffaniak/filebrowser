package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"strconv"
	"time"

	"golang.org/x/crypto/bcrypt"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
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
func shareListHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	var (
		shares []*share.Share
		err    error
	)
	if d.User.Permissions.Admin {
		shares, err = shareStore.All()
	} else {
		shares, err = shareStore.FindByUserID(d.User.ID)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	logger.Debugf("api share/list: user=%q admin=%v shares=%d", d.User.Username, d.User.Permissions.Admin, len(shares))
	return RenderJSON(w, r, shareStore.PrepForFrontend(d.User, r, shares...))
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
func shareGetHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	path := r.URL.Query().Get("path")
	sourceName := r.URL.Query().Get("source")
	if path == "" {
		return http.StatusBadRequest, fmt.Errorf("path is required")
	}
	cleanPath, err := utils.SanitizePath(path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %w", err)
	}
	path = cleanPath
	sourceInfo, ok := config.Server.NameToSource[sourceName] // backend source is path
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source name: %s", sourceName)
	}
	userscope, err := d.User.GetScopeForSourceName(sourceName)
	if err != nil {
		return http.StatusForbidden, err
	}
	scopePath := utils.JoinPathAsUnix(userscope, path)
	idx := indexing.GetIndex(sourceInfo.Name)
	if idx == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source: %s", sourceName)
	}

	logger.Debug("shareGetHandler querying", "sourceName", sourceName, "sourceInfoPath", sourceInfo.Path, "scopePath", scopePath, "userID", d.User.ID)

	s, err := shareStore.Gets(scopePath, sourceInfo.Path, d.User.ID)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error getting share info from server")
	}
	logger.Debug("shareGetHandler result", "sourceName", sourceName, "scopePath", scopePath, "userID", d.User.ID, "count", len(s))
	return RenderJSON(w, r, shareStore.PrepForFrontend(d.User, r, s...))
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
func shareDeleteHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	hash := r.URL.Query().Get("hash")

	if hash == "" {
		return http.StatusBadRequest, nil
	}

	thisShare, err := state.GetShare(hash)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("share not found")
	}
	if !thisShare.UserCanEdit(d.User) {
		return http.StatusForbidden, fmt.Errorf("you are not allowed to delete this share")
	}

	err = state.DeleteShare(hash)
	if err != nil {
		return ErrToStatus(err), err
	}

	activity.RecordShareMutation(r, toActor(d), activitydb.EventShareDelete, hash, thisShare.SourceName, thisShare.Path, nil)
	return ErrToStatus(err), err
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
func sharePatchHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
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

	sanitizedPath, err := utils.SanitizePath(body.Path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %w", err)
	}
	body.Path = sanitizedPath

	// only allow users to update their own shares
	thisShare, err := shareStore.GetByHash(body.Hash)
	if err != nil {
		return ErrToStatus(err), fmt.Errorf("failed to load share: %w", err)
	}
	if !thisShare.UserCanEdit(d.User) {
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
	prepared := shareStore.PrepForFrontend(d.User, r, updatedShare)
	if len(prepared) == 0 {
		return http.StatusInternalServerError, fmt.Errorf("could not prepare share response")
	}
	changes := []activitydb.FieldChange{{
		Field: "path",
		From:  thisShare.Path,
		To:    updatedShare.Path,
	}}
	activity.RecordShareMutation(r, toActor(d), activitydb.EventShareUpdate, body.Hash, updatedShare.SourceName, updatedShare.Path, changes)
	return RenderJSON(w, r, prepared[0])
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
func sharePostHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
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

	hash, status, err2 := sharePasswordFromRequest(req.Password)
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
		beforeShare, getErr := state.GetShare(req.Hash)
		if getErr != nil {
			return http.StatusBadRequest, fmt.Errorf("share not found")
		}
		err = state.UpdateShare(req.Hash, func(link *share.Share) error {
			shouldResetCounts := link.DownloadsLimit != req.DownloadsLimit ||
				link.PerUserDownloadLimit != req.PerUserDownloadLimit

			if err = applySharePasswordUpdate(link, req.Password, stringHash, token); err != nil {
				return err
			}

			preservedPath := link.Path
			preservedSourcePath := link.SourcePath
			preservedPinned := link.PinnedItems
			preservedVersion := link.Version

			share.ApplyPostBodyUpdate(link, &req, expire)

			link.Path = preservedPath
			link.SourcePath = preservedSourcePath
			link.PinnedItems = preservedPinned
			link.Version = preservedVersion
			link.UserID = d.User.ID
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
		changes := activity.ShareUpdateChanges(&beforeShare, updatedShare)
		prepared := shareStore.PrepForFrontend(d.User, r, updatedShare)
		if len(prepared) == 0 {
			return http.StatusInternalServerError, fmt.Errorf("could not prepare share response")
		}
		if len(changes) > 0 {
			activity.RecordShareMutation(r, toActor(d), activitydb.EventShareUpdate, req.Hash, updatedShare.SourceName, updatedShare.Path, changes)
		}
		return RenderJSON(w, r, prepared[0])
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
	userscope, err := d.User.GetScopeForSourceName(source.Name)
	if err != nil {
		return http.StatusForbidden, err
	}
	providedPath := req.Path

	cleanPath, err := utils.SanitizePath(providedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	storedPath := utils.JoinPathAsUnix(userscope, cleanPath)

	if req.ShareType == "upload" && !req.AllowCreate {
		req.AllowCreate = true
	}
	shareLimits := req.ShareLimits
	shareLimits.SourceName = source.Name

	s := &share.Share{
		ShareSettings: share.ShareSettings{
			FrontendShareInfo: req.FrontendShareInfo,
			ShareLimits:       shareLimits,
		},
		ShareColumns: share.ShareColumns{
			Hash:   secureHash,
			Path:   storedPath,
			Expire: expire,
		},
		SourcePath:   source.Path,
		UserID:       d.User.ID,
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

	created, err := shareStore.GetByHash(secureHash)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	prepared := shareStore.PrepForFrontend(d.User, r, created)
	if len(prepared) == 0 {
		return http.StatusInternalServerError, fmt.Errorf("could not prepare share response")
	}
	activity.RecordShareMutation(r, toActor(d), activitydb.EventShareCreate, s.Hash, s.SourceName, s.Path, nil)
	return RenderJSON(w, r, prepared[0])
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
func shareDirectDownloadHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
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

	cleanPath, err := utils.SanitizePath(path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %w", err)
	}
	path = cleanPath

	// Validate source exists
	sourceInfo, ok := config.Server.NameToSource[source]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("invalid source name: %s", source)
	}

	// Get user scope for this source
	userscope, err := d.User.GetScopeForSourceName(source)
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
	existingShares, err := shareStore.Gets(scopePath, sourceInfo.Path, d.User.ID)
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
				return RenderJSON(w, r, response)
			}
		}
	}

	// No matching existing share found, create a new one
	shareLink := &share.Share{
		ShareSettings: share.ShareSettings{
			FrontendShareInfo: share.FrontendShareInfo{
				QuickDownload: true,
			},
			ShareLimits: share.ShareLimits{
				MaxBandwidth:   downloadSpeed,
				DownloadsLimit: downloadCount,
				SourceName:     sourceInfo.Name,
			},
		},
		ShareColumns: share.ShareColumns{
			Hash:   secureHash,
			Path:   scopePath,
			Expire: expire,
		},
		SourcePath: idx.Path,
		UserID:     d.User.ID,
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

	activity.RecordShareMutation(r, toActor(d), activitydb.EventShareCreate, secureHash, snap.SourceName, snap.Path, nil)
	return RenderJSON(w, r, response)
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
func shareInfoHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
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
	frontendShareInfo.SourceURL = shareInfo.SourceURL(d.User)
	frontendShareInfo.CanEditShare = shareInfo.UserCanEdit(d.User)
	if frontendShareInfo.SourceURL != "" {
		frontendShareInfo.SidebarLinks = append(frontendShareInfo.SidebarLinks, users.SidebarLink{
			Name:     "sourceLocation",
			Category: "custom",
			Target:   frontendShareInfo.SourceURL,
		})
	}
	return RenderJSON(w, r, frontendShareInfo)
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

// sharePasswordFromRequest hashes a password for new shares. Nil or empty means no password.
func sharePasswordFromRequest(password *string) ([]byte, int, error) {
	if password == nil || *password == "" {
		return nil, 0, nil
	}
	return getSharePasswordHash(*password)
}

// applySharePasswordUpdate sets or preserves password credentials on share update.
// Nil password omits the field (keep existing); empty string clears; non-empty replaces.
func applySharePasswordUpdate(link *share.Share, password *string, hashedPassword, token string) error {
	if password == nil {
		return nil
	}
	if *password == "" {
		link.PasswordHash = ""
		link.Token = ""
		return nil
	}
	link.PasswordHash = hashedPassword
	link.Token = token
	return nil
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
