package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
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
// @Router /api/resources/preview [get]
func previewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if config.Server.DisablePreviews {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled")
	}
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if path == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid request path")
	}
	if source == "" {
		return http.StatusBadRequest, fmt.Errorf("source is required")
	}
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Path:     path,
		Source:   source,
		AlbumArt: true, // Extract album art for audio previews
		Expand:   true,
	}, store.Access, d.user, store.Share)
	if err != nil {
		logger.Errorf("error getting file info: %v", err)
		return errToStatus(err), err
	}
	d.fileInfo = *fileInfo
	status, err := previewHelperFunc(w, r, d)
	if err != nil {
		// Error already logged in previewHelperFunc or its callees
		return errToStatus(err), err
	}
	return status, nil
}

func rawFileHandler(w http.ResponseWriter, r *http.Request, file iteminfo.ExtendedFileInfo) (int, error) {
	fd, err := os.Open(file.RealPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	setContentDisposition(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}

// getDirectoryPreview returns the previewable file at the given frame index (0–3) for motion preview.
// atPercentage maps to frame: 0→0, 1–25→1, 26–50→2, 51–100→3. The returned file is frameIndex % n where n is the number of previewable items.
func getDirectoryPreview(r *http.Request, d *requestContext, frameIndex int) (*iteminfo.ExtendedFileInfo, error) {
	// Build list of previewable item names in stable order (same as d.fileInfo.Files)
	var previewableNames []string
	for _, item := range d.fileInfo.Files {
		if !item.HasPreview || !iteminfo.ShouldBubbleUpToFolderPreview(item.ItemInfo) {
			continue
		}
		previewableNames = append(previewableNames, item.Name)
	}
	if len(previewableNames) == 0 {
		return nil, fmt.Errorf("no previewable files found in directory")
	}
	// Cycle: 1 image → always 0; 2 images → 0,1,0,1; 3 → 0,1,2,0; 4 → 0,1,2,3
	index := frameIndex % len(previewableNames)
	name := previewableNames[index]

	source := d.fileInfo.Source
	path := utils.JoinPathAsUnix(d.fileInfo.Path, name)
	if d.share != nil {
		sourceInfo, ok := settings.Config.Server.SourceMap[d.share.Source]
		if !ok {
			return nil, fmt.Errorf("source not found for share")
		}
		source = sourceInfo.Name
		path = d.IndexPath + name
	}
	fileInfo, err := files.FileInfoFaster(
		utils.FileOptions{
			Path:     path,
			Source:   source,
			AlbumArt: true,
			Metadata: true,
		}, store.Access, d.user, store.Share)
	if err != nil {
		return nil, err
	}
	tempCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	_, previewErr := preview.GetPreviewForFile(tempCtx, *fileInfo, "small", "", 0)
	cancel()
	if previewErr != nil {
		if !errors.Is(previewErr, context.Canceled) && !errors.Is(previewErr, context.DeadlineExceeded) {
			logger.Debugf("Skipping preview file in directory '%s': %s (error: %v)", d.fileInfo.Name, name, previewErr)
			// Fallback: try first item (frame 0) once so atPercentage=0 still works when another frame fails
			if index != 0 {
				return getDirectoryPreview(r, d, 0)
			}
		}
		return nil, previewErr
	}
	return fileInfo, nil
}

func previewHelperFunc(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	previewSize := r.URL.Query().Get("size")
	if !(previewSize == "large" || previewSize == "original" || previewSize == "xlarge") {
		previewSize = "small"
	}
	if !d.fileInfo.HasPreview {
		return http.StatusBadRequest, fmt.Errorf("this item does not have a preview")
	}

	seekPercentage := 0
	percentage := r.URL.Query().Get("atPercentage")
	if percentage != "" {
		var err error
		seekPercentage, err = strconv.Atoi(percentage)
		if err != nil {
			seekPercentage = 0
		}
		if seekPercentage < 0 || seekPercentage > 100 {
			seekPercentage = 0
		}
	}

	// For directories: map atPercentage to frame index 0–3 for motion preview (cycle over previewable items)
	var dirFrameIndex int
	if d.fileInfo.Type == "directory" {
		switch {
		case seekPercentage <= 0:
			dirFrameIndex = 0
		case seekPercentage <= 25:
			dirFrameIndex = 1
		case seekPercentage <= 50:
			dirFrameIndex = 2
		default:
			dirFrameIndex = 3
		}
		fileInfo, err := getDirectoryPreview(r, d, dirFrameIndex)
		if err != nil {
			logger.Errorf("error getting directory preview: %v", err)
			return http.StatusInternalServerError, err
		}
		d.fileInfo = *fileInfo
		seekPercentage = 0
	}

	setContentDisposition(w, r, d.fileInfo.Name)
	isImage := strings.HasPrefix(d.fileInfo.Type, "image")
	ext := strings.ToLower(filepath.Ext(d.fileInfo.Name))
	resizable := iteminfo.ResizableImageTypes[ext]

	// For small, displayable images (jpg, png, etc.) serve the original to avoid processing.
	const maxSizeForOriginal = 128 * 1024 // 128KB
	if resizable && (config.Server.DisableResize || d.fileInfo.Size < maxSizeForOriginal) && isImage {
		return rawFileHandler(w, r, d.fileInfo)
	}

	officeUrl := ""
	if d.fileInfo.OnlyOfficeId != "" {
		pathUrl := fmt.Sprintf("/api/resources/raw?file=%s&source=%s", url.QueryEscape(d.fileInfo.Path), url.QueryEscape(d.fileInfo.Source))
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
