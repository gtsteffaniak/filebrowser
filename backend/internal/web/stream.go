package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/time/rate"
)

const (
	viewGrantTTL        = 15 * time.Minute
	maxStreamRangeBytes = 4 << 20 // 4 MiB
)

var errStreamRangeInvalid = errors.New("invalid byte range")

type ServeSingleFileOptions struct {
	ForceInline bool
	RangeOnly   bool
}

func viewGrantScope(d *Context, sourceName string) string {
	if d.Share.Hash != "" {
		return d.Share.Hash
	}
	return sourceName
}

func shareSourceName(d *Context) (string, error) {
	if d.Share.Hash == "" {
		return "", fmt.Errorf("not a share context")
	}
	sourceInfo, ok := settings.Config.Server.SourceMap[d.Share.SourcePath]
	if !ok {
		return "", fmt.Errorf("source not found for share")
	}
	return sourceInfo.Name, nil
}

func resolveViewGrantSource(d *Context, r *http.Request) (string, error) {
	if d.Share.Hash != "" {
		if strings.TrimSpace(r.URL.Query().Get("source")) != "" {
			return "", fmt.Errorf("source must not be supplied for share view tokens")
		}
		return shareSourceName(d)
	}
	sourceName := strings.TrimSpace(r.URL.Query().Get("source"))
	if sourceName == "" {
		return "", fmt.Errorf("source is required")
	}
	return sourceName, nil
}

func normalizeViewGrantPath(p string) string {
	return filepath.ToSlash(strings.TrimSpace(p))
}

// shareRelativeDisplayName returns the client-facing filename for a path within a share.
// Single-file shares use file=/ at the share root; the real name comes from the share path.
func shareRelativeDisplayName(d *Context, shareRelativePath string) string {
	name := filepath.Base(normalizeViewGrantPath(shareRelativePath))
	if name != "" && name != "." && name != "/" {
		return name
	}
	if d != nil && d.Share.Hash != "" {
		if shareName := filepath.Base(d.Share.Path); shareName != "" && shareName != "." && shareName != "/" {
			return shareName
		}
	}
	return name
}

func mintViewGrant(d *Context, sourceName string) (string, error) {
	internalSource := sourceName
	if d.Share.Hash != "" {
		var err error
		internalSource, err = shareSourceName(d)
		if err != nil {
			return "", err
		}
	} else if internalSource == "" {
		return "", fmt.Errorf("source is required")
	}
	if !canMintViewToken(d, internalSource) {
		return "", fmt.Errorf("view permission required")
	}
	scope := viewGrantScope(d, internalSource)
	if existing, ok := utils.ViewGrantIndex.Get(scope); ok {
		if grant, ok := utils.ViewGrantsCache.Get(existing); ok {
			if grant.Source == scope && time.Now().Unix() <= grant.ExpiresAt {
				grant.ExpiresAt = time.Now().Add(viewGrantTTL).Unix()
				utils.ViewGrantsCache.Set(existing, grant)
				return existing, nil
			}
		}
	}
	token, err := utils.RandomHex(16)
	if err != nil {
		return "", err
	}
	grant := utils.ViewGrant{
		Source:    scope,
		ExpiresAt: time.Now().Add(viewGrantTTL).Unix(),
	}
	utils.ViewGrantsCache.Set(token, grant)
	utils.ViewGrantIndex.Set(scope, token)
	return token, nil
}

func refreshViewGrant(d *Context, sourceName, existingToken string) (string, int64, error) {
	internalSource := sourceName
	if d.Share.Hash != "" {
		var err error
		internalSource, err = shareSourceName(d)
		if err != nil {
			return "", 0, err
		}
	} else if internalSource == "" {
		return "", 0, fmt.Errorf("source is required")
	}
	scope := viewGrantScope(d, internalSource)
	if existingToken != "" {
		if grant, ok := utils.ViewGrantsCache.Get(existingToken); ok {
			if grant.Source == scope && time.Now().Unix() <= grant.ExpiresAt {
				grant.ExpiresAt = time.Now().Add(viewGrantTTL).Unix()
				utils.ViewGrantsCache.Set(existingToken, grant)
				utils.ViewGrantIndex.Set(scope, existingToken)
				return existingToken, grant.ExpiresAt, nil
			}
		}
	}
	token, err := mintViewGrant(d, internalSource)
	if err != nil {
		return "", 0, err
	}
	grant, ok := utils.ViewGrantsCache.Get(token)
	if !ok {
		return "", 0, fmt.Errorf("view token missing after mint")
	}
	return token, grant.ExpiresAt, nil
}

func requireWebSessionForViewToken(d *Context) error {
	if d.Share.Hash != "" {
		return nil
	}
	if d.Token == "" {
		return fmt.Errorf("view tokens require a web session")
	}
	if _, ok := state.TokenNameForRawToken(d.User, d.Token); ok {
		return fmt.Errorf("view tokens require a web session")
	}
	return nil
}

type viewTokenResponse struct {
	ViewToken string `json:"viewToken"`
	ExpiresAt int64  `json:"expiresAt"`
}

// viewTokenHandler mints or refreshes a source-scoped view grant.
// @Summary Refresh view token
// @Description Mints or extends a source-scoped view token for inline viewing. Authenticated routes require a web session (not a named API token). Public share routes accept anonymous users when the share allows it.
// @Tags Resources
// @Accept json
// @Produce json
// @Param source query string false "Source name or share hash"
// @Param hash query string false "Share hash (public share routes)"
// @Param viewToken query string false "Existing view token to extend"
// @Success 200 {object} viewTokenResponse
// @Failure 403 {object} map[string]string "Missing permission or API token used"
// @Router /api/resources/view-token [post]
// @Router /public/api/resources/view-token [post]
func viewTokenHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if err := requireWebSessionForViewToken(d); err != nil {
		return http.StatusForbidden, err
	}
	if d.Share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("viewing is disabled for upload shares")
	}
	source, err := resolveViewGrantSource(d, r)
	if err != nil {
		return http.StatusBadRequest, err
	}
	existingToken := strings.TrimSpace(r.URL.Query().Get("viewToken"))
	if existingToken == "" && r.Body != nil && r.ContentLength != 0 {
		var body struct {
			ViewToken string `json:"viewToken"`
		}
		if err = json.NewDecoder(r.Body).Decode(&body); err == nil {
			existingToken = strings.TrimSpace(body.ViewToken)
		}
	}
	token, expiresAt, err := refreshViewGrant(d, source, existingToken)
	if err != nil {
		if strings.Contains(err.Error(), "view permission") {
			return http.StatusForbidden, err
		}
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(viewTokenResponse{
		ViewToken: token,
		ExpiresAt: expiresAt,
	}); err != nil {
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func ValidateViewGrant(token string, d *Context, sourceName string) error {
	grant, ok := utils.ViewGrantsCache.Get(token)
	if !ok {
		return fmt.Errorf("invalid or expired view token")
	}
	if time.Now().Unix() > grant.ExpiresAt {
		utils.ViewGrantsCache.Delete(token)
		return fmt.Errorf("view token expired")
	}
	if grant.Source != viewGrantScope(d, sourceName) {
		return fmt.Errorf("view token scope mismatch")
	}
	internalSource := sourceName
	if d.Share.Hash != "" {
		var err error
		internalSource, err = shareSourceName(d)
		if err != nil {
			return err
		}
	}
	perms, err := effectiveFilePerms(d, internalSource)
	if err != nil || !perms.View {
		return fmt.Errorf("view permission required")
	}
	grant.ExpiresAt = time.Now().Add(viewGrantTTL).Unix()
	utils.ViewGrantsCache.Set(token, grant)
	return nil
}

func canMintViewToken(d *Context, source string) bool {
	if d.Share.Hash != "" && d.Share.ShareType == "upload" {
		return false
	}
	perms, err := effectiveFilePerms(d, source)
	return err == nil && perms.View
}

func AttachViewToken(d *Context, source string, file *iteminfo.ExtendedFileInfo) {
	if file == nil || file.Type == "directory" {
		return
	}
	if !canMintViewToken(d, source) {
		return
	}
	token, err := mintViewGrant(d, source)
	if err != nil {
		return
	}
	file.ViewToken = token
}

func AttachViewTokensForDirectory(d *Context, source string, file *iteminfo.ExtendedFileInfo) {
	if file == nil || file.Type != "directory" {
		return
	}
	if !canMintViewToken(d, source) {
		return
	}
	token, err := mintViewGrant(d, source)
	if err != nil {
		return
	}
	for i := range file.Files {
		if file.Files[i].Type == "directory" {
			continue
		}
		file.Files[i].ViewToken = token
	}
}

// streamUseRangeOnly reports whether the stream endpoint must serve capped partial
// content only (never a full-file 200 response). The stream endpoint is media-only.
func streamUseRangeOnly(_ *Context, _ string) bool {
	return true
}

func IsMediaStreamFile(displayFileName string) bool {
	contentType := mime.TypeByExtension(strings.ToLower(filepathExt(displayFileName)))
	return strings.HasPrefix(contentType, "video/") || strings.HasPrefix(contentType, "audio/")
}

func filepathExt(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i:]
	}
	return ""
}

func parseStreamByteRange(rangeHeader string, size int64) (start, end int64, err error) {
	if size <= 0 {
		return 0, 0, errStreamRangeInvalid
	}
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return 0, 0, errStreamRangeInvalid
	}
	spec := strings.TrimPrefix(rangeHeader, "bytes=")
	if spec == "" || strings.Contains(spec, ",") {
		return 0, 0, errStreamRangeInvalid
	}

	dash := strings.Index(spec, "-")
	if dash < 0 {
		return 0, 0, errStreamRangeInvalid
	}
	startStr := strings.TrimSpace(spec[:dash])
	endStr := strings.TrimSpace(spec[dash+1:])

	if startStr == "" {
		// suffix range: bytes=-500
		suffix, parseErr := strconv.ParseInt(endStr, 10, 64)
		if parseErr != nil || suffix <= 0 {
			return 0, 0, errStreamRangeInvalid
		}
		if suffix > size {
			suffix = size
		}
		start = size - suffix
		end = size - 1
		return start, end, nil
	}

	start, err = strconv.ParseInt(startStr, 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, errStreamRangeInvalid
	}

	if endStr == "" {
		end = size - 1
	} else {
		end, err = strconv.ParseInt(endStr, 10, 64)
		if err != nil || end < start {
			return 0, 0, errStreamRangeInvalid
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, nil
}

func capStreamByteRange(start, end int64) (int64, int64) {
	if end-start+1 <= maxStreamRangeBytes {
		return start, end
	}
	return start, start + maxStreamRangeBytes - 1
}

func setStreamResponseHeaders(w http.ResponseWriter, r *http.Request, displayFileName string, size int64) {
	SetContentDisposition(w, r, displayFileName, true)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if contentType := mime.TypeByExtension(strings.ToLower(filepathExt(displayFileName))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
}

func serveStreamByteRange(w http.ResponseWriter, r *http.Request, reader io.ReadSeeker, displayFileName string, size int64) (int, error) {
	if r.Method == http.MethodHead {
		setStreamResponseHeaders(w, r, displayFileName, size)
		return http.StatusOK, nil
	}

	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		setStreamResponseHeaders(w, r, displayFileName, size)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		return http.StatusRequestedRangeNotSatisfiable, fmt.Errorf("stream requires byte range requests")
	}

	start, end, err := parseStreamByteRange(rangeHeader, size)
	if err != nil {
		setStreamResponseHeaders(w, r, displayFileName, size)
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		return http.StatusRequestedRangeNotSatisfiable, fmt.Errorf("invalid byte range")
	}
	start, end = capStreamByteRange(start, end)

	chunkSize := end - start + 1
	if _, err := reader.Seek(start, io.SeekStart); err != nil {
		return http.StatusInternalServerError, err
	}

	setStreamResponseHeaders(w, r, displayFileName, chunkSize)
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.WriteHeader(http.StatusPartialContent)

	if _, err := io.CopyN(w, reader, chunkSize); err != nil && !errors.Is(err, io.EOF) {
		return http.StatusPartialContent, err
	}
	return http.StatusPartialContent, nil
}

// ServeSingleFile opens one file and streams it with Range support via http.ServeContent.
func ServeSingleFile(w http.ResponseWriter, r *http.Request, d *Context, source string, scopedFilePath string, displayFileName string, opts ServeSingleFileOptions) (int, error) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusInternalServerError, fmt.Errorf("source %s is not available", source)
	}
	permUser := d.User.Username
	if d.Share.Hash != "" {
		permUser = d.ShareUser.Username
	}

	if !state.AccessPermitted(idx.Path, utils.IndexPathFromNormalized(scopedFilePath, true), permUser) {
		logger.Debugf("user %s denied access to path %s", permUser, scopedFilePath)
		return http.StatusForbidden, fmt.Errorf("access denied to path %s", scopedFilePath)
	}

	realPath, _, err := idx.GetRealPath(scopedFilePath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	isOnlyOffice := IsOnlyOfficeCompatibleFile(displayFileName) && settings.Config.Integrations.OnlyOffice.Url != ""
	var documentId string
	var logContext *OnlyOfficeLogContext
	if isOnlyOffice {
		documentId, _ = GetOnlyOfficeId(realPath)
		if documentId != "" {
			logContext = GetOnlyOfficeLogContext(documentId)
		}
	}

	fd, err := os.Open(realPath)
	if err != nil {
		if isOnlyOffice && logContext != nil {
			SendOnlyOfficeLogEvent(logContext, "ERROR", "download",
				fmt.Sprintf("OnlyOffice download failed - could not open file: %s - %v", scopedFilePath, err))
		}
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	fileInfo, err := fd.Stat()
	if err != nil {
		if isOnlyOffice && logContext != nil {
			SendOnlyOfficeLogEvent(logContext, "ERROR", "download",
				fmt.Sprintf("OnlyOffice download failed - could not get file info: %s - %v", scopedFilePath, err))
		}
		return http.StatusInternalServerError, err
	}
	if fileInfo.IsDir() {
		return http.StatusForbidden, fmt.Errorf("cannot stream a directory")
	}

	if isOnlyOffice && logContext != nil {
		logger.Infof("OnlyOffice Server is downloading file: %s (documentId: %s)", scopedFilePath, documentId)
		SendOnlyOfficeLogEvent(logContext, "INFO", "download",
			fmt.Sprintf("OnlyOffice Server downloading file: %s", scopedFilePath))
	}

	SetContentDisposition(w, r, displayFileName, opts.ForceInline)
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	var reader io.ReadSeeker = fd
	if d.Share.Hash != "" && d.Share.MaxBandwidth > 0 {
		limit := rate.Limit(d.Share.MaxBandwidth * 1024)
		burst := d.Share.MaxBandwidth * 1024
		reader = NewThrottledReadSeeker(fd, limit, burst, r.Context())
	}

	if opts.RangeOnly {
		return serveStreamByteRange(w, r, reader, displayFileName, fileInfo.Size())
	}

	srw := &ResponseWriterWrapper{ResponseWriter: w}
	http.ServeContent(srw, r, displayFileName, fileInfo.ModTime(), reader)
	recordStatus := srw.StatusCode
	if recordStatus == 0 {
		recordStatus = http.StatusOK
	}
	return recordStatus, nil
}

func streamFilesHandler(w http.ResponseWriter, r *http.Request, d *Context, source string, scopedFileList []string) (int, error) {
	if len(scopedFileList) != 1 {
		return http.StatusForbidden, fmt.Errorf("stream supports single file only")
	}
	scopedFilePath := scopedFileList[0]
	displayName := filepath.Base(scopedFilePath)
	return ServeSingleFile(w, r, d, source, scopedFilePath, displayName, ServeSingleFileOptions{
		ForceInline: true,
		RangeOnly:   streamUseRangeOnly(d, displayName),
	})
}

// streamHandler serves inline audio/video content with a valid viewToken.
// @Summary Stream content of a single media file for inline viewing
// @Description Returns raw file bytes for inline UI viewing in capped byte ranges. Requires a viewToken minted by GET /resources. Media files must use Range requests; full-file GET responses are rejected. Never counts toward download limits or activity.
// @Tags Resources
// @Accept json
// @Param source query string true "Source name for the file (required)"
// @Param file query string true "File path"
// @Param viewToken query string true "Opaque view grant token from file metadata"
// @Success 200 {file} file "Raw file content (inline)"
// @Failure 403 {object} map[string]string "Missing or invalid view token"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/media/stream [get]
func streamHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if r.URL.Query().Get("archiveToken") != "" || r.URL.Query().Get("algo") != "" {
		return http.StatusForbidden, fmt.Errorf("archives not supported on stream endpoint")
	}
	source := r.URL.Query().Get("source")
	fileList := r.URL.Query()["file"]
	if len(fileList) != 1 {
		return http.StatusForbidden, fmt.Errorf("stream supports single file only")
	}
	token := r.URL.Query().Get("viewToken")
	if token == "" {
		return http.StatusForbidden, fmt.Errorf("view token required")
	}
	cleanPath, err := utils.SanitizePath(fileList[0])
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
	}
	if err = ValidateViewGrant(token, d, source); err != nil {
		return http.StatusForbidden, err
	}
	if !IsMediaStreamFile(filepath.Base(cleanPath)) {
		return http.StatusForbidden, fmt.Errorf("stream endpoint supports audio and video only")
	}

	userscope, err := d.User.GetScopeForSourceName(source)
	if err != nil {
		return http.StatusForbidden, err
	}
	scopedPath := utils.JoinPathAsUnix(userscope, cleanPath)
	return streamFilesHandler(w, r, d, source, []string{scopedPath})
}

// publicStreamHandler serves inline audio/video content from a public share with a valid viewToken.
// @Summary Stream a single media file from a public share for inline viewing
// @Description Returns raw file bytes for inline UI viewing in capped byte ranges on a share link. Requires viewToken from GET /public/api/resources. Media files must use Range requests. Does not count toward download limits.
// @Tags Resources
// @Accept json
// @Produce octet-stream
// @Param hash query string true "Share hash for authentication"
// @Param file query string true "File path within the share"
// @Param viewToken query string true "Opaque view grant token from share file metadata"
// @Success 200 {file} file "Raw file content (inline)"
// @Failure 403 {object} map[string]string "Missing or invalid view token"
// @Failure 404 {object} map[string]string "Share or file not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /public/api/media/stream [get]
func publicStreamHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if d.Share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("streaming is disabled for upload shares")
	}
	if r.URL.Query().Get("archiveToken") != "" || r.URL.Query().Get("algo") != "" {
		return http.StatusForbidden, fmt.Errorf("archives not supported on stream endpoint")
	}
	files := r.URL.Query()["file"]
	if len(files) != 1 {
		return http.StatusForbidden, fmt.Errorf("stream supports single file only")
	}
	token := r.URL.Query().Get("viewToken")
	if token == "" {
		return http.StatusForbidden, fmt.Errorf("view token required")
	}
	cleanFile, err := utils.SanitizePath(files[0])
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
	}
	sourceInfo, ok := settings.Config.Server.SourceMap[d.Share.SourcePath]
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("source not found for share")
	}
	if err = ValidateViewGrant(token, d, ""); err != nil {
		return http.StatusForbidden, err
	}
	if !IsMediaStreamFile(shareRelativeDisplayName(d, cleanFile)) {
		return http.StatusForbidden, fmt.Errorf("stream endpoint supports audio and video only")
	}
	scopedPath := utils.JoinPathAsUnix(d.Share.Path, cleanFile)
	status, err := streamFilesHandler(w, r, d, sourceInfo.Name, []string{scopedPath})
	if err != nil {
		if status == http.StatusForbidden {
			return http.StatusForbidden, fmt.Errorf("access denied")
		}
		return status, fmt.Errorf("error streaming file")
	}
	return status, nil
}

// ResolveDisplayFileList returns client-facing paths for activity logging.
func ResolveDisplayFileList(d *Context, source string, fileList []string) []string {
	if d.Share.Hash != "" {
		display := make([]string, 0, len(fileList))
		sharePrefix := strings.TrimSuffix(d.Share.Path, "/")
		for _, p := range fileList {
			p = strings.TrimPrefix(p, sharePrefix)
			p = strings.TrimPrefix(p, "/")
			display = append(display, p)
		}
		return display
	}
	userscope, err := d.User.GetScopeForSourceName(source)
	if err != nil {
		return append([]string(nil), fileList...)
	}
	display := make([]string, 0, len(fileList))
	for _, p := range fileList {
		rel := strings.TrimPrefix(p, userscope)
		rel = strings.TrimPrefix(rel, "/")
		display = append(display, rel)
	}
	return display
}
