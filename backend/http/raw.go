package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/time/rate"
)

// throttledReadSeeker is a wrapper around an io.ReadSeeker that throttles the reading speed.
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

func setContentDisposition(w http.ResponseWriter, r *http.Request, fileName string) {
	dispositionType := "attachment"
	if r.URL.Query().Get("inline") == "true" {
		dispositionType = "inline"
	}

	// standard: ASCII-only safe fallback
	asciiFileName := toASCIIFilename(fileName)
	// RFC 5987: UTF-8 encoded
	encodedFileName := url.PathEscape(fileName)

	// Always set both filename (ASCII) and filename* (UTF-8) for maximum compatibility (RFC 6266)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q; filename*=utf-8''%s", dispositionType, asciiFileName, encodedFileName))
}

// rawHandler serves the raw content of a file, multiple files, or directory in various formats.
// @Summary Get raw content of a file, multiple files, or directory
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
// @Router /api/resources/raw [get]
func rawHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	fileList := r.URL.Query()["file"]

	// Rule 1: Validate all user-provided file paths to prevent path traversal
	for i, filePath := range fileList {
		cleanPath, err := utils.SanitizeUserPath(filePath)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid file path: %v", err)
		}
		fileList[i] = cleanPath
	}

	return rawFilesHandler(w, r, d, source, fileList)
}

func rawFilesHandler(w http.ResponseWriter, r *http.Request, d *requestContext, source string, fileList []string) (int, error) {
	if !d.user.Permissions.Download && d.share == nil {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to download")
	}

	if len(fileList) == 0 {
		return http.StatusBadRequest, fmt.Errorf("no files specified")
	}

	firstFilePath := fileList[0]
	var err error
	var userscope string
	fileName := filepath.Base(firstFilePath)

	// Check if this is an OnlyOffice file early for error logging
	isOnlyOffice := isOnlyOfficeCompatibleFile(fileName) && config.Integrations.OnlyOffice.Url != ""
	var documentId string
	var logContext *OnlyOfficeLogContext

	if d.share == nil {
		userscope, err = d.user.GetScopeForSourceName(source)
		if err != nil {
			// Send OnlyOffice error log if this was an OnlyOffice file
			if isOnlyOffice {
				// Try to get document ID for error logging
				idx := indexing.GetIndex(source)
				if idx != nil {
					tempPath := utils.JoinPathAsUnix(userscope, firstFilePath)
					if realPath, _, realErr := idx.GetRealPath(tempPath); realErr == nil {
						if docId, _ := getOnlyOfficeId(realPath); docId != "" {
							if ctx := getOnlyOfficeLogContext(docId); ctx != nil {
								sendOnlyOfficeLogEvent(ctx, "ERROR", "download",
									fmt.Sprintf("OnlyOffice download failed - source not available: %s - %v", firstFilePath, err))
							}
						}
					}
				}
			}
			return http.StatusForbidden, err
		}
		firstFilePath = utils.JoinPathAsUnix(userscope, firstFilePath)
	}
	// For shares, the path is already correctly resolved by publicRawHandler
	idx := indexing.GetIndex(source)
	if idx == nil {
		// Send OnlyOffice error log if this was an OnlyOffice file
		if isOnlyOffice {
			sendOnlyOfficeLogEvent(logContext, "ERROR", "download",
				fmt.Sprintf("OnlyOffice download failed - source index not available: %s", source))
		}
		return http.StatusInternalServerError, fmt.Errorf("source %s is not available", source)
	}
	realPath, isDir, err := idx.GetRealPath(firstFilePath)
	if err != nil {
		// Send OnlyOffice error log if this was an OnlyOffice file
		if isOnlyOffice {
			if docId, _ := getOnlyOfficeId(realPath); docId != "" {
				if ctx := getOnlyOfficeLogContext(docId); ctx != nil {
					sendOnlyOfficeLogEvent(ctx, "ERROR", "download",
						fmt.Sprintf("OnlyOffice download failed - could not resolve path: %s - %v", firstFilePath, err))
				}
			}
		}
		return http.StatusInternalServerError, err
	}
	// Compute estimated download size (for single-file branch and archive size check)
	estimatedSize, err := computeArchiveSize(source, fileList, d)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	// ** Single file download with Content-Length **
	if len(fileList) == 1 && !isDir {
		// Get document ID and log context for OnlyOffice downloads
		if isOnlyOffice {
			documentId, _ = getOnlyOfficeId(realPath)
			if documentId != "" {
				logContext = getOnlyOfficeLogContext(documentId)
			}
		}

		// Verify access control before opening the file (direct rule check)
		if d.share == nil && store.Access != nil {
			if !store.Access.Permitted(idx.Path, firstFilePath, d.user.Username) {
				logger.Debugf("user %s denied access to path %s", d.user.Username, firstFilePath)
				// Send OnlyOffice error log if this was an OnlyOffice download
				if isOnlyOffice && logContext != nil {
					sendOnlyOfficeLogEvent(logContext, "ERROR", "download",
						fmt.Sprintf("OnlyOffice download failed - access denied by rule: %s", firstFilePath))
				}
				return http.StatusForbidden, fmt.Errorf("access denied to path %s", firstFilePath)
			}
		}

		fd, err2 := os.Open(realPath)
		if err2 != nil {
			// Send OnlyOffice error log if this was an OnlyOffice download
			if isOnlyOffice && logContext != nil {
				sendOnlyOfficeLogEvent(logContext, "ERROR", "download",
					fmt.Sprintf("OnlyOffice download failed - could not open file: %s - %v", firstFilePath, err2))
			}
			return http.StatusInternalServerError, err2
		}
		defer fd.Close()

		// Get file size
		fileInfo, err2 := fd.Stat()
		if err2 != nil {
			// Send OnlyOffice error log if this was an OnlyOffice download
			if isOnlyOffice && logContext != nil {
				sendOnlyOfficeLogEvent(logContext, "ERROR", "download",
					fmt.Sprintf("OnlyOffice download failed - could not get file info: %s - %v", firstFilePath, err2))
			}
			return http.StatusInternalServerError, err2
		}

		// Send success log for OnlyOffice downloads
		if isOnlyOffice && logContext != nil {
			logger.Infof("OnlyOffice Server is downloading file: %s (documentId: %s)",
				firstFilePath, documentId)

			sendOnlyOfficeLogEvent(logContext, "INFO", "download",
				fmt.Sprintf("OnlyOffice Server downloading file: %s", firstFilePath))
		}

		// Set headers
		setContentDisposition(w, r, fileName)
		w.Header().Set("Cache-Control", "private")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		sizeInMB := estimatedSize / 1024 / 1024
		// if larger than 500 MB, log it
		if sizeInMB > 500 {
			logger.Debugf("User %v is downloading large (%d MB) file: %v", d.user.Username, sizeInMB, fileName)
		}
		// serve content allows for range requests.
		// video scrubbing, etc.
		// Note: http.ServeContent will respect our already-set Content-Disposition header
		var reader io.ReadSeeker = fd
		if d.share != nil && d.share.MaxBandwidth > 0 {
			// convert KB/s to B/s
			limit := rate.Limit(d.share.MaxBandwidth * 1024)
			// burst size can be the same as limit
			burst := d.share.MaxBandwidth * 1024
			reader = newThrottledReadSeeker(fd, limit, burst, r.Context())
		}
		http.ServeContent(w, r, fileName, fileInfo.ModTime(), reader)
		return 200, nil
	}

	// ** Archive (ZIP/TAR.GZ) handling ** — delegate to archive package
	return BuildAndStreamArchive(w, r, d, source, fileList)
}

// isOnlyOfficeCompatibleFile checks if a file extension is supported by OnlyOffice
func isOnlyOfficeCompatibleFile(fileName string) bool {
	return iteminfo.IsOnlyOffice(fileName)
}
