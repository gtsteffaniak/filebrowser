package web

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"golang.org/x/time/rate"
)

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
		if waitErr := tw.limiter.WaitN(tw.ctx, n); waitErr != nil {
			if err == nil {
				err = waitErr
			}
		}
	}
	return
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

func RawFilesHandler(w http.ResponseWriter, r *http.Request, d *Context, source string, fileList []string) (int, error) {
	if !d.User.Permissions.Download && d.Share.Hash == "" {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to download")
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
