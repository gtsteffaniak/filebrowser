package http

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
)

type FileCache interface {
	Store(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, key string) error
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
// @Router /api/preview [get]
func previewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = settings.Config.Server.DefaultSource.Name
	}
	previewSize := r.URL.Query().Get("size")
	if previewSize != "small" {
		previewSize = "large"
	}
	if path == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid request path")
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Path:   utils.JoinPathAsUnix(userscope, path),
		Modify: d.user.Permissions.Modify,
		Source: source,
		Expand: true,
	})
	if err != nil {
		return errToStatus(err), err
	}
	if fileInfo.Type == "directory" {
		return http.StatusBadRequest, fmt.Errorf("can't create preview for directory")
	}
	setContentDisposition(w, r, fileInfo.Name)
	if !preview.AvailablePreview(fileInfo) {
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", fileInfo.Type)
	}

	if (previewSize == "large" && !config.Server.ResizePreview) ||
		(previewSize == "small" && !config.Server.EnableThumbnails) {
		return rawFileHandler(w, r, fileInfo)
	}
	pathUrl := fmt.Sprintf("/api/raw?files=%s::%s", source, path)
	rawUrl := pathUrl
	if config.Server.InternalUrl != "" {
		rawUrl = config.Server.InternalUrl + pathUrl
	}
	rawUrl = rawUrl + "&auth=" + d.token
	previewImg, err := preview.GetPreviewForFile(fileInfo, previewSize, rawUrl)
	// Unsupported extensions directly return the raw data
	if err == preview.ErrUnsupportedFormat {
		return rawFileHandler(w, r, fileInfo)
	}
	// todo concele this error
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, fileInfo.RealPath, fileInfo.ModTime, bytes.NewReader(previewImg))
	return 0, nil
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
