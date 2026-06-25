package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"golang.org/x/time/rate"
)

// throttledReadSeeker is a wrapper around an io.ReadSeeker that throttles the reading speed.
// Used for single-file downloads and for share archive downloads (see serveArchiveWithServeContent);
// archives are built to a temp file first, then bytes to the client are limited on read.
type throttledReadSeeker struct {
	rs      io.ReadSeeker
	limiter *rate.Limiter
	ctx     context.Context
}

// newThrottledReadSeeker creates a new throttledReadSeeker.
func newThrottledReadSeeker(rs io.ReadSeeker, limit rate.Limit, burst int, ctx context.Context) *throttledReadSeeker {
	return &throttledReadSeeker{
		rs:      rs,
		limiter: rate.NewLimiter(limit, burst),
		ctx:     ctx,
	}
}

func (r *throttledReadSeeker) Read(p []byte) (n int, err error) {
	n, err = r.rs.Read(p)
	if n > 0 {
		if waitErr := r.limiter.WaitN(r.ctx, n); waitErr != nil {
			// The original error (like io.EOF) is potentially more important
			// than the context error.
			if err == nil {
				err = waitErr
			}
		}
	}
	return
}

func (r *throttledReadSeeker) Seek(offset int64, whence int) (int64, error) {
	return r.rs.Seek(offset, whence)
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
		if waitErr := tw.limiter.WaitN(tw.ctx, n); waitErr != nil {
			if err == nil {
				err = waitErr
			}
		}
	}
	return
}

// toASCIIFilename converts a filename to ASCII-safe format by replacing non-ASCII characters with underscores
func toASCIIFilename(fileName string) string {
	var result strings.Builder
	for _, r := range fileName {
		if r > 127 {
			// Replace non-ASCII characters with underscore
			result.WriteRune('_')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func setContentDisposition(w http.ResponseWriter, r *http.Request, fileName string, forceInline bool) {
	dispositionType := "attachment"
	if forceInline || r.URL.Query().Get("inline") == "true" {
		dispositionType = "inline"
		// Inline SVG (and similar) can execute embedded scripts when opened as a top-level document; match upstream filebrowser mitigation.
		w.Header().Set("Content-Security-Policy", "script-src 'none'")
	}

	// standard: ASCII-only safe fallback
	asciiFileName := toASCIIFilename(fileName)
	// RFC 5987: UTF-8 encoded
	encodedFileName := url.PathEscape(fileName)

	// Always set both filename (ASCII) and filename* (UTF-8) for maximum compatibility (RFC 6266)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q; filename*=utf-8''%s", dispositionType, asciiFileName, encodedFileName))
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
func downloadHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
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

	return rawFilesHandler(w, r, d, source, fileList)
}

func rawFilesHandler(w http.ResponseWriter, r *http.Request, d *requestContext, source string, fileList []string) (int, error) {
	if !d.user.Permissions.Download && d.share.Hash == "" {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to download")
	}

	if len(fileList) == 0 && d.share.Hash == "" {
		return http.StatusBadRequest, fmt.Errorf("no files specified")
	}

	firstFilePath := fileList[0]
	displayFileList := resolveDisplayFileList(d, source, fileList)
	var err error
	var status int
	var userscope string
	fileName := filepath.Base(firstFilePath)

	// modify all filepaths for user scope
	if d.share.Hash == "" {
		userscope, err = d.user.GetScopeForSourceName(source)
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
		status, err = serveSingleFile(w, r, d, source, firstFilePath, fileName, serveSingleFileOptions{forceInline: forceInline})
		if downloadResponseRecordsActivity(status, err) {
			recordDownloadActivity(r, d, source, displayFileList)
		}
		return status, err
	}

	status, err = BuildAndStreamArchive(w, r, d, source, fileList)
	if status == 0 && err == nil {
		status = http.StatusOK
	}
	if downloadResponseRecordsActivity(status, err) {
		recordDownloadActivity(r, d, source, displayFileList)
	}
	return status, err
}

func downloadResponseRecordsActivity(status int, err error) bool {
	return err == nil && status >= http.StatusOK && status < http.StatusMultipleChoices
}

// isOnlyOfficeCompatibleFile checks if a file extension is supported by OnlyOffice
func isOnlyOfficeCompatibleFile(fileName string) bool {
	return iteminfo.IsOnlyOffice(fileName)
}
