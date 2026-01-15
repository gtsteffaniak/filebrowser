package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
	"github.com/gtsteffaniak/go-logger/logger"
)

type FileCache interface {
	Store(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, key string) error
}

// isClientCancellation checks if an error is due to client cancellation (navigation away)
func isClientCancellation(ctx context.Context, err error) bool {
	// Check context state first
	if ctx.Err() == context.Canceled {
		return true
	}

	// Check if the error chain contains context cancellation
	return errors.Is(err, context.Canceled)
}

// previewHandler handles the preview request for images.
// @Summary Get image preview
// @Description Returns a preview image based on the requested path and size.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "File path of the image to preview"
// @Param size query string false "Preview size ('small' or 'large'). Default is based on server config."
// @Success 200 {file} file "Preview image content"
// @Failure 202 {object} map[string]string "Download permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 415 {object} map[string]string "Unsupported file type for preview"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Preview generation not implemented"
// @Router /api/preview [get]
func previewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if config.Server.DisablePreviews {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled")
	}
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if path == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid request path")
	}
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Path:     path,
		Source:   source,
		AlbumArt: true, // Extract album art for audio previews
		Expand:   true,
	}, store.Access, d.user)
	if err != nil {
		return errToStatus(err), err
	}
	d.fileInfo = *fileInfo
	return previewHelperFunc(w, r, d)
}

func rawFileHandler(w http.ResponseWriter, r *http.Request, file iteminfo.ExtendedFileInfo) (int, error) {
	idx := indexing.GetIndex(file.Source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source not found: %s", file.Source)
	}
	realPath, _, _ := idx.GetRealPath(file.Path)
	fd, err := os.Open(realPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	setContentDisposition(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}

// getDirectoryPreview finds a valid preview file within a directory.
func getDirectoryPreview(r *http.Request, d *requestContext) (*iteminfo.ExtendedFileInfo, error) {
	var lastErr error
	for _, item := range d.fileInfo.Files {
		if !item.HasPreview || !iteminfo.ShouldBubbleUpToFolderPreview(item.ItemInfo) {
			continue
		}
		source := d.fileInfo.Source
		path := utils.JoinPathAsUnix(d.fileInfo.Path, item.Name)
		if d.share != nil {
			sourceInfo, ok := settings.Config.Server.SourceMap[d.share.Source]
			if !ok {
				return nil, fmt.Errorf("source not found for share")
			}
			source = sourceInfo.Name
			path = d.IndexPath + item.Name
		}
		fileInfo, err := files.FileInfoFaster(
			utils.FileOptions{
				Path:     path,
				Source:   source,
				AlbumArt: true, // Extract album art for audio previews
				Metadata: true,
			}, store.Access, d.user)
		if err != nil {
			lastErr = err
			continue // Try next file if this one fails
		}
		// Try to generate preview to verify the file is not corrupted
		tempCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		_, previewErr := preview.GetPreviewForFile(tempCtx, *fileInfo, "small", "", 0)
		cancel()
		if previewErr != nil {
			// Skip context errors (timeout or cancellation) - they're not corruption issues
			if !errors.Is(previewErr, context.Canceled) && !errors.Is(previewErr, context.DeadlineExceeded) {
				// File might be corrupted, try next one
				logger.Debugf("Skipping preview file in directory '%s': %s (error: %v)",
					d.fileInfo.Name, item.Name, previewErr)
			} else {
				// if it is a context error, return the error
				// don't keep trying
				return nil, previewErr
			}
			lastErr = previewErr
			continue
		}
		return fileInfo, nil
	}
	// If we exhausted all files and none worked, return the last error
	if lastErr != nil {
		return nil, fmt.Errorf("no valid preview files found in directory: %w", lastErr)
	}

	return nil, fmt.Errorf("no previewable files found in directory")
}

func previewHelperFunc(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	previewSize := r.URL.Query().Get("size")
	if !(previewSize == "large" || previewSize == "original") {
		previewSize = "small"
	}
	if !d.fileInfo.HasPreview {
		return http.StatusBadRequest, fmt.Errorf("this item does not have a preview")
	}

	if d.fileInfo.Type == "directory" {
		// Get extended file info of first previewable item in directory
		fileInfo, err := getDirectoryPreview(r, d)
		if err != nil {
			fmt.Println("error getting directory preview", err)
			return http.StatusInternalServerError, err
		}
		fmt.Println("fileInfo", fileInfo.Name, fileInfo.Path)
		d.fileInfo = *fileInfo
	}

	setContentDisposition(w, r, d.fileInfo.Name)
	isImage := strings.HasPrefix(d.fileInfo.Type, "image")
	ext := strings.ToLower(filepath.Ext(d.fileInfo.Name))
	resizable := iteminfo.ResizableImageTypes[ext]
	if (!resizable || config.Server.DisableResize) && isImage {
		return rawFileHandler(w, r, d.fileInfo)
	}
	seekPercentage := 0
	percentage := r.URL.Query().Get("atPercentage")
	if percentage != "" {
		var err error
		// convert string to int
		seekPercentage, err = strconv.Atoi(percentage)
		if err != nil {
			seekPercentage = 10
		}
		if seekPercentage < 0 || seekPercentage > 100 {
			seekPercentage = 10
		}
	}

	officeUrl := ""
	if d.fileInfo.OnlyOfficeId != "" {
		pathUrl := fmt.Sprintf("/api/raw?files=%s::%s", d.fileInfo.Source, d.fileInfo.Path)
		pathUrl = pathUrl + "&auth=" + d.token
		if settings.Config.Server.InternalUrl != "" {
			officeUrl = config.Server.InternalUrl + pathUrl
		} else {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			officeUrl = scheme + "://" + r.Host + pathUrl
		}
	}
	// Use the context from the request context (which includes timeout)
	ctx := r.Context()
	if d.ctx != nil {
		ctx = d.ctx
	}

	previewImg, err := preview.GetPreviewForFile(ctx, d.fileInfo, previewSize, officeUrl, seekPercentage)
	if err != nil {
		// Check if it was a context cancellation (client navigated away)
		if isClientCancellation(ctx, err) {
			// Return 200 to avoid error logging - client cancellation is normal
			return http.StatusOK, nil
		}

		// Check if it was a context timeout (server-side timeout)
		if ctx.Err() == context.DeadlineExceeded || errors.Is(err, context.DeadlineExceeded) {
			logger.Errorf("Preview timeout for file '%s' after 60 seconds", d.fileInfo.Name)
			return http.StatusRequestTimeout, fmt.Errorf("preview generation timed out after 60 seconds")
		}

		// Log detailed error information for actual server errors
		logger.Errorf("Preview generation failed for file '%s' (type: %s, size: %s, seek: %d%%): %v",
			d.fileInfo.Name, d.fileInfo.Type, previewSize, seekPercentage, err)

		return http.StatusInternalServerError, err
	}
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, d.fileInfo.Name+"-preview.jpg", d.fileInfo.ModTime, bytes.NewReader(previewImg))
	return 0, nil
}
