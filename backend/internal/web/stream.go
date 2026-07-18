package web

import (
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

func normalizeViewGrantPath(p string) string {
	return filepath.ToSlash(strings.TrimSpace(p))
}

func mintViewGrant(d *Context, source, filePath string) (string, error) {
	token, err := utils.RandomHex(16)
	if err != nil {
		return "", err
	}
	grant := utils.ViewGrant{
		UserID:    d.User.ID,
		ShareHash: d.Share.Hash,
		Source:    source,
		Path:      normalizeViewGrantPath(filePath),
		ExpiresAt: time.Now().Add(viewGrantTTL).Unix(),
	}
	utils.ViewGrantsCache.Set(token, grant)
	return token, nil
}

func ValidateViewGrant(token string, d *Context, source, filePath string) error {
	grant, ok := utils.ViewGrantsCache.Get(token)
	if !ok {
		return fmt.Errorf("invalid or expired view token")
	}
	if time.Now().Unix() > grant.ExpiresAt {
		utils.ViewGrantsCache.Delete(token)
		return fmt.Errorf("view token expired")
	}
	if grant.UserID != d.User.ID {
		return fmt.Errorf("view token viewer mismatch")
	}
	if grant.ShareHash != d.Share.Hash {
		return fmt.Errorf("view token share mismatch")
	}
	if grant.Source != source {
		return fmt.Errorf("view token source mismatch")
	}
	if grant.Path != normalizeViewGrantPath(filePath) {
		return fmt.Errorf("view token path mismatch")
	}
	perms, err := effectiveFilePerms(d, source)
	if err != nil || !perms.View {
		return fmt.Errorf("view permission required")
	}
	return nil
}

func canMintViewToken(d *Context, source string) bool {
	perms, err := effectiveFilePerms(d, source)
	return err == nil && perms.View
}

func AttachViewToken(d *Context, source, filePath string, file *iteminfo.ExtendedFileInfo) {
	if file == nil || file.Type == "directory" {
		return
	}
	if !canMintViewToken(d, source) {
		return
	}
	token, err := mintViewGrant(d, source, filePath)
	if err != nil {
		return
	}
	file.ViewToken = token
}

func indexFilePath(dirPath, name string) string {
	dirPath = normalizeViewGrantPath(dirPath)
	if dirPath == "" || dirPath == "/" {
		return "/" + name
	}
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}
	return dirPath + name
}

func AttachViewTokensForDirectory(d *Context, source, dirPath string, file *iteminfo.ExtendedFileInfo) {
	if file == nil || file.Type != "directory" {
		return
	}
	if !canMintViewToken(d, source) {
		return
	}
	for i := range file.Files {
		if file.Files[i].Type == "directory" {
			continue
		}
		childPath := indexFilePath(dirPath, file.Files[i].Name)
		token, err := mintViewGrant(d, source, childPath)
		if err != nil {
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

	SetContentDisposition(w, r, displayFileName, true)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if contentType := mime.TypeByExtension(strings.ToLower(filepathExt(displayFileName))); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	w.Header().Set("Content-Length", strconv.FormatInt(chunkSize, 10))
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
	if err = ValidateViewGrant(token, d, source, cleanPath); err != nil {
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
	if err = ValidateViewGrant(token, d, sourceInfo.Name, cleanFile); err != nil {
		return http.StatusForbidden, err
	}
	if !IsMediaStreamFile(filepath.Base(cleanFile)) {
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
