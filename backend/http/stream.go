package http

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/time/rate"
)

type serveSingleFileOptions struct {
	forceInline bool
	rangeOnly   bool
}

// serveSingleFile opens one file and streams it with Range support via http.ServeContent.
func serveSingleFile(w http.ResponseWriter, r *http.Request, d *requestContext, source string, scopedFilePath string, displayFileName string, opts serveSingleFileOptions) (int, error) {
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusInternalServerError, fmt.Errorf("source %s is not available", source)
	}
	permUser := d.user.Username
	if d.share.Hash != "" {
		permUser = d.shareUser.Username
	}

	if !accessStore.Permitted(idx.Path, utils.IndexPathFromNormalized(scopedFilePath, true), permUser) {
		logger.Debugf("user %s denied access to path %s", permUser, scopedFilePath)
		return http.StatusForbidden, fmt.Errorf("access denied to path %s", scopedFilePath)
	}

	realPath, _, err := idx.GetRealPath(scopedFilePath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	isOnlyOffice := isOnlyOfficeCompatibleFile(displayFileName) && config.Integrations.OnlyOffice.Url != ""
	var documentId string
	var logContext *OnlyOfficeLogContext
	if isOnlyOffice {
		documentId, _ = getOnlyOfficeId(realPath)
		if documentId != "" {
			logContext = getOnlyOfficeLogContext(documentId)
		}
	}

	fd, err := os.Open(realPath)
	if err != nil {
		if isOnlyOffice && logContext != nil {
			sendOnlyOfficeLogEvent(logContext, "ERROR", "download",
				fmt.Sprintf("OnlyOffice download failed - could not open file: %s - %v", scopedFilePath, err))
		}
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	fileInfo, err := fd.Stat()
	if err != nil {
		if isOnlyOffice && logContext != nil {
			sendOnlyOfficeLogEvent(logContext, "ERROR", "download",
				fmt.Sprintf("OnlyOffice download failed - could not get file info: %s - %v", scopedFilePath, err))
		}
		return http.StatusInternalServerError, err
	}
	if fileInfo.IsDir() {
		return http.StatusForbidden, fmt.Errorf("cannot stream a directory")
	}

	if isOnlyOffice && logContext != nil {
		logger.Infof("OnlyOffice Server is downloading file: %s (documentId: %s)", scopedFilePath, documentId)
		sendOnlyOfficeLogEvent(logContext, "INFO", "download",
			fmt.Sprintf("OnlyOffice Server downloading file: %s", scopedFilePath))
	}

	setContentDisposition(w, r, displayFileName, opts.forceInline)
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	var reader io.ReadSeeker = fd
	if d.share.Hash != "" && d.share.MaxBandwidth > 0 {
		limit := rate.Limit(d.share.MaxBandwidth * 1024)
		burst := d.share.MaxBandwidth * 1024
		reader = newThrottledReadSeeker(fd, limit, burst, r.Context())
	}

	if opts.rangeOnly {
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

func streamFilesHandler(w http.ResponseWriter, r *http.Request, d *requestContext, source string, scopedFileList []string) (int, error) {
	if len(scopedFileList) != 1 {
		return http.StatusForbidden, fmt.Errorf("stream supports single file only")
	}
	scopedFilePath := scopedFileList[0]
	displayName := filepath.Base(scopedFilePath)
	return serveSingleFile(w, r, d, source, scopedFilePath, displayName, serveSingleFileOptions{
		forceInline: true,
		rangeOnly:   streamUseRangeOnly(d, displayName),
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
func streamHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
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
	if err = validateViewGrant(token, d, source, cleanPath); err != nil {
		return http.StatusForbidden, err
	}
	if !isMediaStreamFile(filepath.Base(cleanPath)) {
		return http.StatusForbidden, fmt.Errorf("stream endpoint supports audio and video only")
	}

	userscope, err := d.user.GetScopeForSourceName(source)
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
func publicStreamHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.share.ShareType == "upload" {
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
	sourceInfo, ok := config.Server.SourceMap[d.share.SourcePath]
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("source not found for share")
	}
	if err = validateViewGrant(token, d, sourceInfo.Name, cleanFile); err != nil {
		return http.StatusForbidden, err
	}
	if !isMediaStreamFile(filepath.Base(cleanFile)) {
		return http.StatusForbidden, fmt.Errorf("stream endpoint supports audio and video only")
	}
	scopedPath := utils.JoinPathAsUnix(d.share.Path, cleanFile)
	status, err := streamFilesHandler(w, r, d, sourceInfo.Name, []string{scopedPath})
	if err != nil {
		if status == http.StatusForbidden {
			return http.StatusForbidden, fmt.Errorf("access denied")
		}
		return status, fmt.Errorf("error streaming file")
	}
	return status, nil
}

// resolveDisplayFileList returns client-facing paths for activity logging.
func resolveDisplayFileList(d *requestContext, source string, fileList []string) []string {
	if d.share.Hash != "" {
		display := make([]string, 0, len(fileList))
		sharePrefix := strings.TrimSuffix(d.share.Path, "/")
		for _, p := range fileList {
			p = strings.TrimPrefix(p, sharePrefix)
			p = strings.TrimPrefix(p, "/")
			display = append(display, p)
		}
		return display
	}
	userscope, err := d.user.GetScopeForSourceName(source)
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
