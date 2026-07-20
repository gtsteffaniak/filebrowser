package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/time/rate"
)

// waitLimiterBytes applies rate limiting in chunks no larger than the limiter burst.
func waitLimiterBytes(ctx context.Context, lim *rate.Limiter, n int) error {
	for remaining := n; remaining > 0; {
		chunk := remaining
		if burst := lim.Burst(); burst > 0 && chunk > burst {
			chunk = burst
		}
		if chunk < 1 {
			chunk = 1
		}
		if err := lim.WaitN(ctx, chunk); err != nil {
			return err
		}
		remaining -= chunk
	}
	return nil
}

// throttledWriter wraps an io.Writer and rate-limits outbound bytes (streaming archives to the client).
type throttledWriter struct {
	w       io.Writer
	limiter *rate.Limiter
	ctx     context.Context
}

func newThrottledWriter(w io.Writer, limit rate.Limit, burst int, ctx context.Context) *throttledWriter {
	return &throttledWriter{
		w:       w,
		limiter: rate.NewLimiter(limit, burst),
		ctx:     ctx,
	}
}

func (tw *throttledWriter) Write(p []byte) (n int, err error) {
	n, err = tw.w.Write(p)
	if n > 0 {
		if waitErr := waitLimiterBytes(tw.ctx, tw.limiter, n); waitErr != nil && err == nil {
			err = waitErr
		}
	}
	return
}

type throttledReadSeeker struct {
	rs      io.ReadSeeker
	limiter *rate.Limiter
	ctx     context.Context
}

// NewThrottledReadSeeker rate-limits reads from an io.ReadSeeker.
func NewThrottledReadSeeker(rs io.ReadSeeker, limit rate.Limit, burst int, ctx context.Context) io.ReadSeeker {
	return &throttledReadSeeker{
		rs:      rs,
		limiter: rate.NewLimiter(limit, burst),
		ctx:     ctx,
	}
}

func (r *throttledReadSeeker) Read(p []byte) (n int, err error) {
	n, err = r.rs.Read(p)
	if n > 0 {
		if waitErr := waitLimiterBytes(r.ctx, r.limiter, n); waitErr != nil && err == nil {
			err = waitErr
		}
	}
	return
}

func (r *throttledReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return r.rs.Seek(offset, whence)
}

// downloadHandler serves the raw content of a file, multiple files, or directory in various formats.
// @Summary Download content of a file, multiple files, or directory
// @Description Returns the raw content of a file, multiple files, or a directory. Supports downloading files as archives in various formats.
// @Description
// @Description **Filename Encoding:**
// @Description - The Content-Disposition header will always include both:
// @Description   1. `filename="..."`: An ASCII-safe version of the filename for compatibility.
// @Description   2. `filename*=utf-8"...`: The full UTF-8 encoded filename (RFC 6266/5987) for modern clients.
// @Description
// @Description **Multiple Files:**
// @Description - Use repeated query parameters: `?file=file1.txt&file=file2.txt&file=file3.txt`
// @Description - This supports filenames containing commas and special characters
// @Tags Resources
// @Accept json
// @Param source query string true "Source name for the files (required)"
// @Param file query []string true "File path (can be repeated for multiple files)"
// @Param inline query bool false "If true, sets 'Content-Disposition' to 'inline'. Otherwise, defaults to 'attachment'."
// @Param algo query string false "Compression algorithm for archiving multiple files or directories. Options: 'zip' and 'tar.gz'. Default is 'zip'."
// @Success 200 {file} file "Raw file or directory content, or archive for multiple files"
// @Failure 202 {object} map[string]string "Modify permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File or directory not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources/download [get]
func downloadHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	source := r.URL.Query().Get("source")
	fileList := r.URL.Query()["file"]

	// Rule 1: Validate all user-provided file paths to prevent path traversal
	for i, filePath := range fileList {
		cleanPath, err := utils.SanitizePath(filePath)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
		}
		fileList[i] = cleanPath
	}

	return RawFilesHandler(w, r, d, source, fileList)
}

// publicDownloadHandler serves the raw content of a file, multiple files, or directory via a public share.
// @Summary Download files from a public share
// @Description Downloads raw content from a public share. Supports single files, multiple files, or directories as archives. Enforces download limits (global or per-user) and blocks anonymous users when per-user limits are enabled.
// @Description
// @Description **Multiple Files:**
// @Description - Use repeated query parameters: `?file=file1.txt&file=file2.txt&file=file3.txt`
// @Description - This supports filenames containing commas and special characters
// @Tags Resources
// @Accept json
// @Produce octet-stream
// @Param hash query string true "Share hash for authentication"
// @Param file query []string true "File path (can be repeated for multiple files)"
// @Param inline query bool false "If true, sets 'Content-Disposition' to 'inline'. Otherwise, defaults to 'attachment'."
// @Param algo query string false "Compression algorithm for archiving multiple files or directories. Options: 'zip' and 'tar.gz'. Default is 'zip'."
// @Success 200 {file} file "Raw file or directory content, or archive for multiple files"
// @Failure 400 {object} map[string]string "Invalid request path or encoding"
// @Failure 403 {object} map[string]string "Download limit reached, anonymous access blocked, or share unavailable"
// @Failure 404 {object} map[string]string "Share not found or file not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Downloads disabled for upload shares"
// @Router /public/api/resources/download [get]
func publicDownloadHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if d.Share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("downloads are disabled for upload shares")
	}

	if d.Share.DisableDownload {
		return http.StatusForbidden, fmt.Errorf("downloads are not allowed for this share")
	}

	if !d.Share.PerUserDownloadLimit && d.Share.DownloadsLimit > 0 && d.Share.Downloads >= d.Share.DownloadsLimit {
		return http.StatusForbidden, fmt.Errorf("share downloads limit reached")
	}

	if d.Share.PerUserDownloadLimit {
		if d.User.Username == "anonymous" {
			return http.StatusForbidden, fmt.Errorf("anonymous downloads are not allowed with per-user limits")
		}
		if d.Share.HasReachedUserLimit(d.User.Username) {
			return http.StatusForbidden, fmt.Errorf("user download limit reached for this share")
		}
	}

	files := r.URL.Query()["file"]
	if len(files) == 0 {
		files = []string{"/"}
	}

	sourceInfo, ok := settings.Config.Server.SourceMap[d.Share.SourcePath]
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("source not found for share")
	}
	actualSourceName := sourceInfo.Name

	fileList := []string{}
	for _, file := range files {
		cleanFile, err := utils.SanitizePath(file)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
		}
		filePath := utils.JoinPathAsUnix(d.Share.Path, cleanFile)
		fileList = append(fileList, filePath)
	}

	status, err := RawFilesHandler(w, r, d, actualSourceName, fileList)
	if err != nil {
		if err == errors.ErrDownloadNotAllowed {
			return http.StatusForbidden, errors.ErrDownloadNotAllowed
		}
		logger.Errorf("public share handler: error processing filelist: %v with error %v", files, err)
		return status, fmt.Errorf("error processing filelist: %v", files)
	}
	if downloadResponseRecordsActivity(status, err) {
		if recErr := state.RecordShareDownload(d.Share.Hash, d.User.Username); recErr != nil {
			logger.Errorf("public share handler: failed to record download for share %s: %v", d.Share.Hash, recErr)
		}
	}
	return status, nil
}

func RawFilesHandler(w http.ResponseWriter, r *http.Request, d *Context, source string, fileList []string) (int, error) {
	if d.Share.Hash == "" {
		filePerms, err := effectiveFilePerms(d, source)
		if err != nil {
			return http.StatusForbidden, err
		}
		if !filePerms.Download {
			return http.StatusForbidden, fmt.Errorf("user is not allowed to download")
		}
	}

	if len(fileList) == 0 && d.Share.Hash == "" {
		return http.StatusBadRequest, fmt.Errorf("no files specified")
	}

	firstFilePath := fileList[0]
	displayFileList := ResolveDisplayFileList(d, source, fileList)
	var err error
	var status int
	var userscope string
	fileName := filepath.Base(firstFilePath)

	// modify all filepaths for user scope
	if d.Share.Hash == "" {
		userscope, err = d.User.GetScopeForSourceName(source)
		if err != nil {
			return http.StatusForbidden, err
		}
		for i, filePath := range fileList {
			fileList[i] = utils.JoinPathAsUnix(userscope, filePath)
		}
	}
	firstFilePath = fileList[0]
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusInternalServerError, fmt.Errorf("source %s is not available", source)
	}
	_, isDir, err := idx.GetRealPath(firstFilePath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if len(fileList) == 1 && !isDir {
		forceInline := r.URL.Query().Get("inline") == "true"
		status, err = ServeSingleFile(w, r, d, source, firstFilePath, fileName, ServeSingleFileOptions{ForceInline: forceInline})
		if downloadResponseRecordsActivity(status, err) {
			activity.RecordDownload(r, toActor(d), source, displayFileList)
		}
		return status, err
	}

	status, err = BuildAndStreamArchive(w, r, d, source, fileList)
	if status == 0 && err == nil {
		status = http.StatusOK
	}
	if downloadResponseRecordsActivity(status, err) {
		activity.RecordDownload(r, toActor(d), source, displayFileList)
	}
	return status, err
}

func downloadResponseRecordsActivity(status int, err error) bool {
	return err == nil && status >= http.StatusOK && status < http.StatusMultipleChoices
}
