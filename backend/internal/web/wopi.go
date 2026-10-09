package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// WOPI (Web Application Open Platform Interface) lets an online editor such as
// Collabora Online edit files that FileBrowser stores. FileBrowser is the WOPI
// host: the SPA asks /api/wopi/session for an editor URL and an access token,
// form-posts the token into an iframe, and from then on the editor's server
// calls back /wopi/files/{id}[/contents] with that token to read the file
// (CheckFileInfo, GetFile), lock it, and save it (PutFile).
//
// Every WOPI request is authenticated by the access token alone: it carries no
// session cookie, and must therefore bypass any SSO proxy in front of
// FileBrowser. Permissions are re-evaluated on each request, so revoking a
// user's write access takes effect mid-session, not at token expiry.

// wopiTimestampLayout is how LastModifiedTime is rendered. Collabora echoes it
// back in X-COOL-WOPI-Timestamp on save, so it must round-trip exactly.
const wopiTimestampLayout = "2006-01-02T15:04:05.000000Z"

// coolStatusDocChanged is Collabora's status code for "the document changed in
// storage since it was loaded"; it then lets the user overwrite or reload.
const coolStatusDocChanged = 1010

type wopiSessionResponse struct {
	ActionURL      string `json:"actionUrl"`      // editor URL the iframe form posts to
	AccessToken    string `json:"accessToken"`    // posted as access_token
	AccessTokenTTL int64  `json:"accessTokenTtl"` // posted as access_token_ttl: absolute expiry, epoch milliseconds
	Mode           string `json:"mode"`           // "edit" or "view"
	Product        string `json:"product"`        // collabora, onlyoffice or generic
}

// wopiSessionHandler prepares an editing session for one file.
//
// @Summary Open a file in the WOPI editor
// @Description Returns the editor URL and the access token the browser posts to it.
// @Tags Office
// @Produce json
// @Param source query string true "Source name"
// @Param path query string true "File path"
// @Success 200 {object} wopiSessionResponse
// @Failure 400 {object} map[string]string "Missing or invalid parameters"
// @Failure 403 {object} map[string]string "No view permission"
// @Failure 415 {object} map[string]string "The editor does not handle this file type"
// @Failure 503 {object} map[string]string "Editor discovery unavailable"
// @Router /api/wopi/session [get]
// @Security ApiKeyAuth
func wopiSessionHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if !wopiEnabled() {
		return http.StatusNotFound, errors.New("wopi integration is not configured")
	}
	disc, err := currentWopiDiscovery(r.Context())
	if err != nil {
		logger.Errorf("wopi: %v", err)
		return http.StatusServiceUnavailable, errors.New("wopi editor discovery is unavailable")
	}

	source := r.URL.Query().Get("source")
	if source == "" || r.URL.Query().Get("path") == "" {
		return http.StatusBadRequest, errors.New("missing required parameters: source and path")
	}
	path, err := utils.SanitizePath(r.URL.Query().Get("path"))
	if err != nil {
		return http.StatusBadRequest, err
	}
	sourceInfo, ok := settings.Config.Server.NameToSource[source]
	if !ok {
		return http.StatusNotFound, errors.New("source not found")
	}
	source = sourceInfo.Name

	perms, err := effectiveFilePerms(d, source)
	if err != nil || !perms.View {
		return http.StatusForbidden, errors.New("user is not allowed to view files in this source")
	}
	fi, err := files.FileInfoFaster(utils.FileOptions{
		Path:           path,
		Source:         source,
		FollowSymlinks: true,
	}, d.User)
	if err != nil {
		return ErrToStatus(err), err
	}
	if fi.Type == "directory" {
		return http.StatusBadRequest, errors.New("a directory cannot be opened in an editor")
	}

	ext := strings.TrimPrefix(filepath.Ext(fi.Name), ".")
	canWrite := perms.Modify && !settings.Config.Integrations.OnlyOffice.ViewOnly
	urlsrc, ok := disc.editorURL(ext, canWrite)
	if !ok {
		return http.StatusUnsupportedMediaType, fmt.Errorf("the editor does not handle .%s files", ext)
	}
	if _, editable := disc.Actions[strings.ToLower(ext)][wopiActionEdit]; !editable {
		canWrite = false
	}

	fileID := wopiFileID(source, fi.RealPath)
	now := time.Now()
	token, expires, err := mintWopiToken(wopiClaims{
		FileID: fileID,
		Source: source,
		Path:   path,
		UserID: d.User.ID,
		Write:  canWrite,
		Origin: wopiBrowserOrigin(r),
	}, now)
	if err != nil {
		logger.Errorf("wopi: could not sign access token: %v", err)
		return http.StatusInternalServerError, errors.New("could not create wopi session")
	}

	wopiSrc := joinOnlyOfficeAPIURL(onlyOfficeFileBrowserBaseURL(r), "wopi/files/"+fileID)
	actionURL, err := buildWopiEditorURL(urlsrc, wopiSrc, disc.Product, d.User.Locale)
	if err != nil {
		logger.Errorf("wopi: %v", err)
		return http.StatusInternalServerError, errors.New("could not build editor url")
	}

	return RenderJSON(w, r, wopiSessionResponse{
		ActionURL:      actionURL,
		AccessToken:    token,
		AccessTokenTTL: expires.UnixMilli(),
		Mode:           utils.Ternary(canWrite, wopiActionEdit, wopiActionView),
		Product:        disc.Product,
	})
}

// wopiBrowserOrigin is the origin the SPA runs on, the only one allowed to
// exchange postMessages with the editor iframe.
func wopiBrowserOrigin(r *http.Request) string {
	if ext := strings.TrimSuffix(settings.Config.Http.ExternalUrl, "/"); ext != "" {
		return ext
	}
	return requestSchemeForPublicURL(r) + "://" + requestHost(r)
}

// wopiRequest is what withWopiToken resolves before a WOPI operation runs.
type wopiRequest struct {
	claims   *wopiClaims
	user     *users.User
	ctx      *Context
	realPath string
	name     string
	canWrite bool
	download bool
}

type wopiHandlerFunc func(w http.ResponseWriter, r *http.Request, req *wopiRequest) int

// wopiUserByID resolves the user an access token was minted for; a variable so
// tests can stand in for the user store.
var wopiUserByID = state.GetUserByID

// withWopiToken authenticates a WOPI request by its access_token and resolves
// the file it designates, re-checking the user's permissions on the way.
// WOPI clients only read status codes, so errors are logged, not rendered.
func withWopiToken(fn wopiHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := serveWopi(w, r, fn)
		if status != 0 {
			w.WriteHeader(status)
		}
	}
}

func serveWopi(w http.ResponseWriter, r *http.Request, fn wopiHandlerFunc) int {
	if !wopiEnabled() {
		return http.StatusNotFound
	}
	fileID := r.PathValue("id")
	claims, err := parseWopiToken(r.URL.Query().Get("access_token"), fileID)
	if err != nil {
		logger.Debugf("wopi: rejected request on %s: %v", fileID, err)
		return http.StatusUnauthorized
	}
	user, err := wopiUserByID(claims.UserID)
	if err != nil {
		logger.Debugf("wopi: token user %d not found: %v", claims.UserID, err)
		return http.StatusUnauthorized
	}
	SetUserInResponseWriter(w, &user)
	ctx := &Context{User: &user, Ctx: r.Context()}
	perms, err := effectiveFilePerms(ctx, claims.Source)
	if err != nil || !perms.View {
		logger.Debugf("wopi: user %s lost view permission on source %s", user.Username, claims.Source)
		return http.StatusUnauthorized
	}
	fi, err := files.FileInfoFaster(utils.FileOptions{
		Path:           claims.Path,
		Source:         claims.Source,
		FollowSymlinks: true,
	}, &user)
	if err != nil {
		return http.StatusNotFound
	}
	// The token names a path; the id pins where the file was when the session
	// opened. A file moved since is not the same document. A file replaced in
	// place is: PutFile reports it as changed in storage and the editor lets
	// the user resolve it.
	if wopiFileID(claims.Source, fi.RealPath) != fileID {
		return http.StatusNotFound
	}
	return fn(w, r, &wopiRequest{
		claims:   claims,
		user:     &user,
		ctx:      ctx,
		realPath: fi.RealPath,
		name:     fi.Name,
		canWrite: claims.Write && perms.Modify,
		download: perms.Download,
	})
}

// wopiCheckFileInfo is the union of the CheckFileInfo properties Collabora and
// OnlyOffice read; each ignores the ones it does not know.
type wopiCheckFileInfo struct {
	BaseFileName            string `json:"BaseFileName"`
	BreadcrumbDocName       string `json:"BreadcrumbDocName"`
	Size                    int64  `json:"Size"`
	Version                 string `json:"Version"`
	LastModifiedTime        string `json:"LastModifiedTime"`
	OwnerId                 string `json:"OwnerId"`
	UserId                  string `json:"UserId"`
	UserFriendlyName        string `json:"UserFriendlyName"`
	UserCanWrite            bool   `json:"UserCanWrite"`
	ReadOnly                bool   `json:"ReadOnly"`
	UserCanNotWriteRelative bool   `json:"UserCanNotWriteRelative"`
	UserCanRename           bool   `json:"UserCanRename"`
	SupportsLocks           bool   `json:"SupportsLocks"`
	SupportsGetLock         bool   `json:"SupportsGetLock"`
	SupportsUpdate          bool   `json:"SupportsUpdate"`
	SupportsRename          bool   `json:"SupportsRename"`
	SupportsDeleteFile      bool   `json:"SupportsDeleteFile"`
	PostMessageOrigin       string `json:"PostMessageOrigin,omitempty"`
	DisablePrint            bool   `json:"DisablePrint,omitempty"`
	DisableExport           bool   `json:"DisableExport,omitempty"`
	DisableCopy             bool   `json:"DisableCopy,omitempty"`
	HidePrintOption         bool   `json:"HidePrintOption,omitempty"`
	HideExportOption        bool   `json:"HideExportOption,omitempty"`
	EnableOwnerTermination  bool   `json:"EnableOwnerTermination"`
}

func wopiVersion(stat os.FileInfo) string {
	return strconv.FormatInt(stat.ModTime().UnixNano(), 36) + "-" + strconv.FormatInt(stat.Size(), 36)
}

func wopiTimestamp(t time.Time) string {
	return t.UTC().Format(wopiTimestampLayout)
}

func buildWopiCheckFileInfo(req *wopiRequest, stat os.FileInfo) wopiCheckFileInfo {
	userID := strconv.FormatUint(req.user.ID, 10)
	return wopiCheckFileInfo{
		BaseFileName:      req.name,
		BreadcrumbDocName: req.name,
		Size:              stat.Size(),
		Version:           wopiVersion(stat),
		LastModifiedTime:  wopiTimestamp(stat.ModTime()),
		// FileBrowser has no notion of a file owner distinct from who can
		// reach it; the session user stands in for one.
		OwnerId:          userID,
		UserId:           userID,
		UserFriendlyName: req.user.Username,
		UserCanWrite:     req.canWrite,
		ReadOnly:         !req.canWrite,
		// Save As, rename and delete from inside the editor are not offered.
		UserCanNotWriteRelative: true,
		UserCanRename:           false,
		SupportsLocks:           true,
		SupportsGetLock:         true,
		SupportsUpdate:          true,
		SupportsRename:          false,
		SupportsDeleteFile:      false,
		PostMessageOrigin:       req.claims.Origin,
		DisablePrint:            !req.download,
		DisableExport:           !req.download,
		DisableCopy:             !req.download,
		HidePrintOption:         !req.download,
		HideExportOption:        !req.download,
		EnableOwnerTermination:  false,
	}
}

// wopiCheckFileInfoHandler answers GET /wopi/files/{id}.
func wopiCheckFileInfoHandler(w http.ResponseWriter, r *http.Request, req *wopiRequest) int {
	stat, err := os.Stat(req.realPath)
	if err != nil {
		return http.StatusNotFound
	}
	status, err := RenderJSON(w, r, buildWopiCheckFileInfo(req, stat))
	if err != nil {
		logger.Errorf("wopi: CheckFileInfo: %v", err)
		return status
	}
	return 0
}

// wopiGetFileHandler answers GET /wopi/files/{id}/contents.
func wopiGetFileHandler(w http.ResponseWriter, r *http.Request, req *wopiRequest) int {
	f, err := os.Open(req.realPath)
	if err != nil {
		return http.StatusNotFound
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		return http.StatusNotFound
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))
	w.Header().Set("X-WOPI-ItemVersion", wopiVersion(stat))
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, f); err != nil {
		logger.Debugf("wopi: GetFile %s interrupted: %v", req.name, err)
	}
	return 0
}

// wopiFilesPostHandler dispatches POST /wopi/files/{id} on X-WOPI-Override.
func wopiFilesPostHandler(w http.ResponseWriter, r *http.Request, req *wopiRequest) int {
	fileID := req.claims.FileID
	lockID := r.Header.Get("X-WOPI-Lock")
	override := r.Header.Get("X-WOPI-Override")

	conflict := func(current string) int {
		w.Header().Set("X-WOPI-Lock", current)
		w.Header().Set("X-WOPI-LockFailureReason", utils.Ternary(current == "", "file is not locked", "file is locked by another session"))
		return http.StatusConflict
	}

	switch override {
	case "GET_LOCK":
		w.Header().Set("X-WOPI-Lock", wopiLocks.get(fileID))
		return http.StatusOK
	case "LOCK", "REFRESH_LOCK", "UNLOCK":
		if !req.canWrite {
			return http.StatusUnauthorized
		}
		if lockID == "" {
			return http.StatusBadRequest
		}
		var ok bool
		var current string
		switch {
		case override == "LOCK" && r.Header.Get("X-WOPI-OldLock") != "":
			ok, current = wopiLocks.unlockAndRelock(fileID, r.Header.Get("X-WOPI-OldLock"), lockID)
		case override == "LOCK":
			ok, current = wopiLocks.lock(fileID, lockID)
		case override == "REFRESH_LOCK":
			ok, current = wopiLocks.refresh(fileID, lockID)
		default:
			ok, current = wopiLocks.unlock(fileID, lockID)
		}
		if !ok {
			return conflict(current)
		}
		if stat, err := os.Stat(req.realPath); err == nil {
			w.Header().Set("X-WOPI-ItemVersion", wopiVersion(stat))
		}
		return http.StatusOK
	default:
		// PUT_RELATIVE, RENAME_FILE, DELETE, PUT_USER_INFO: advertised as
		// unsupported in CheckFileInfo.
		return http.StatusNotImplemented
	}
}

// wopiPutFileHandler answers POST /wopi/files/{id}/contents (PutFile).
func wopiPutFileHandler(w http.ResponseWriter, r *http.Request, req *wopiRequest) int {
	if r.Header.Get("X-WOPI-Override") != "PUT" {
		return http.StatusBadRequest
	}
	if !req.canWrite {
		return http.StatusUnauthorized
	}
	if ok, current := wopiLocks.checkPut(req.claims.FileID, r.Header.Get("X-WOPI-Lock")); !ok {
		w.Header().Set("X-WOPI-Lock", current)
		w.Header().Set("X-WOPI-LockFailureReason", "file is locked by another session")
		return http.StatusConflict
	}

	// Same key as the OnlyOffice callback, so the two integrations never write
	// the same file at once.
	unlock := lockOnlyOfficeDoc(req.claims.Source + "|" + req.claims.Path)
	defer unlock()

	stat, err := os.Stat(req.realPath)
	if err != nil {
		return http.StatusNotFound
	}
	if wopiStorageChanged(r.Header.Get("X-COOL-WOPI-Timestamp"), stat.ModTime()) {
		logger.Infof("wopi: %s changed in storage since it was opened, asking the editor to resolve", req.name)
		if _, err = RenderJSON(w, r, map[string]int{"COOLStatusCode": coolStatusDocChanged}, http.StatusConflict); err != nil {
			logger.Debugf("wopi: PutFile conflict response: %v", err)
		}
		return 0
	}

	// Receive the whole document before touching the original: a dropped
	// connection must not leave a truncated file behind.
	spool, err := os.CreateTemp(settings.Config.Server.CacheDir, "wopi-put-*")
	if err != nil {
		spool, err = os.CreateTemp("", "wopi-put-*")
	}
	if err != nil {
		logger.Errorf("wopi: PutFile: could not create spool file: %v", err)
		return http.StatusInternalServerError
	}
	defer func() {
		spool.Close()
		os.Remove(spool.Name())
	}()
	if _, err = io.Copy(spool, r.Body); err != nil {
		logger.Errorf("wopi: PutFile: receiving %s: %v", req.name, err)
		return http.StatusInternalServerError
	}
	if _, err = spool.Seek(0, io.SeekStart); err != nil {
		return http.StatusInternalServerError
	}

	scope, err := req.user.GetScopeForSourceName(req.claims.Source)
	if err != nil {
		return http.StatusUnauthorized
	}
	if err = files.WriteFile(req.claims.Source, utils.JoinPathAsUnix(scope, req.claims.Path), spool); err != nil {
		logger.Errorf("wopi: PutFile: writing %s: %v", req.name, err)
		return http.StatusInternalServerError
	}

	stat, err = os.Stat(req.realPath)
	if err != nil {
		return http.StatusInternalServerError
	}
	logger.Debugf("wopi: saved %s for %s", req.name, req.user.Username)
	w.Header().Set("X-WOPI-ItemVersion", wopiVersion(stat))
	if _, err := RenderJSON(w, r, map[string]string{"LastModifiedTime": wopiTimestamp(stat.ModTime())}); err != nil {
		logger.Debugf("wopi: PutFile response: %v", err)
	}
	return 0
}

// wopiStorageChanged reports whether the file changed since the editor last
// saw it. Collabora sends back the LastModifiedTime it was given; a missing
// header means the editor asks to overwrite unconditionally.
func wopiStorageChanged(header string, modTime time.Time) bool {
	if header == "" {
		return false
	}
	seen, err := time.Parse(time.RFC3339Nano, header)
	if err != nil {
		return false
	}
	return !seen.Equal(modTime.UTC().Truncate(time.Microsecond))
}

// redactAccessToken hides WOPI access tokens from logged URLs.
func redactAccessToken(rawQuery string) string {
	if !strings.Contains(rawQuery, "access_token=") {
		return rawQuery
	}
	parts := strings.Split(rawQuery, "&")
	for i, p := range parts {
		if strings.HasPrefix(p, "access_token=") {
			parts[i] = "access_token=REDACTED"
		}
	}
	return strings.Join(parts, "&")
}
