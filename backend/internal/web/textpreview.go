package web

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/textpreview"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// textPreviewHandler returns a size-capped text preview for text-like files.
// @Summary Get text preview
// @Description Returns the first ~16KB of a text-like file as JSON. Markdown, HTML, and CSS are returned as raw text; RTF is converted to HTML. The response is NOT sanitized: clients must sanitize HTML before rendering it. Responses are not cached on the server.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "File path of the text file to preview"
// @Param source query string true "Source name"
// @Success 200 {object} textpreview.Result "Text preview content"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 403 {object} map[string]string "User is not allowed to view files in this source"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 415 {object} map[string]string "File type is not supported for text preview, or the file is binary"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Previews are disabled"
// @Router /api/resources/preview/text [get]
func textPreviewHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if settings.Config.Server.DisablePreviews {
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
	filePerms, err := effectiveFilePerms(d, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	if !filePerms.View {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to view files in this source")
	}
	fileInfo, err := files.FileInfoFaster(utils.FileOptions{
		Path:           path,
		Source:         source,
		FollowSymlinks: true,
	}, d.User)
	if err != nil {
		logger.Errorf("error getting file info: %v", err)
		return ErrToStatus(err), err
	}
	return serveTextPreview(w, r, fileInfo)
}

// publicTextPreviewHandler returns a text preview for a file inside a public share.
// @Summary Get text preview from a public share
// @Description Returns the first ~16KB of a text-like file in a public share as JSON. Same content rules as the authenticated endpoint. Not available for upload-only shares.
// @Tags Resources
// @Accept json
// @Produce json
// @Param hash query string true "Share hash for authentication"
// @Param path query string true "File path within the share to preview"
// @Success 200 {object} textpreview.Result "Text preview content"
// @Failure 403 {object} map[string]string "Share unavailable or access denied"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 415 {object} map[string]string "File type is not supported for text preview, or the file is binary"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Previews disabled globally, for this share, or for upload shares"
// @Router /public/api/resources/preview/text [get]
func publicTextPreviewHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	if settings.Config.Server.DisablePreviews || d.Share.DisableThumbnails {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled")
	}
	if d.Share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled for upload shares")
	}
	return serveTextPreview(w, r, &d.FileInfo)
}

// serveTextPreview writes the JSON text preview for an already-resolved file.
// Nothing is written to disk and no server-side cache is used.
func serveTextPreview(w http.ResponseWriter, r *http.Request, info *iteminfo.ExtendedFileInfo) (int, error) {
	if info.Type == "directory" {
		return http.StatusUnsupportedMediaType, fmt.Errorf("directories have no text preview")
	}
	kind := iteminfo.TextPreviewKind(strings.ToLower(filepath.Ext(info.Name)))
	if kind == "" {
		return http.StatusUnsupportedMediaType, fmt.Errorf("unsupported file type for text preview")
	}
	result, err := textpreview.Read(info.RealPath, kind)
	if err != nil {
		switch {
		case errors.Is(err, textpreview.ErrNotText):
			return http.StatusUnsupportedMediaType, err
		case errors.Is(err, os.ErrNotExist):
			return http.StatusNotFound, fmt.Errorf("file not found")
		}
		logger.Errorf("text preview failed for '%s': %v", info.Name, err)
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Cache-Control", "private, no-cache")
	return RenderJSON(w, r, result)
}
