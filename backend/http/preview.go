package http

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

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
// @Failure 501 {object} map[string]string "Preview generation not implemented"
// @Router /api/preview [get]
func previewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if config.Server.DisablePreviews {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled")
	}
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	var err error
	// decode url encoded source name
	source, err = url.QueryUnescape(source)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
	}
	if path == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid request path")
	}
	userscope, err := settings.GetScopeFromSourceName(d.user.Scopes, source)
	if err != nil {
		return http.StatusForbidden, err
	}
	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Access:   store.Access,
		Username: d.user.Username,
		Path:     utils.JoinPathAsUnix(userscope, path),
		Source:   source,
		Content:  true,
	})
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

func previewHelperFunc(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	previewSize := r.URL.Query().Get("size")
	if !(previewSize == "large" || previewSize == "original") {
		previewSize = "small"
	}
	if d.fileInfo.Type == "directory" && !d.fileInfo.HasPreview {
		return http.StatusBadRequest, fmt.Errorf("This folder does not have a preview")
	}
	if d.fileInfo.Type == "directory" {
		// get extended file info of first previewable item in directory
		for _, item := range d.fileInfo.Files {
			if preview.AvailablePreview(item) {
				fileInfo, err := files.FileInfoFaster(
					iteminfo.FileOptions{
						Path:    utils.JoinPathAsUnix(d.fileInfo.Path, item.Name),
						Source:  d.fileInfo.Source,
						Content: true,
					})
				if err != nil {
					return http.StatusInternalServerError, err
				}
				d.fileInfo = *fileInfo
				break
			}
		}
	}

	setContentDisposition(w, r, d.fileInfo.Name)
	isImage := strings.HasPrefix(d.fileInfo.Type, "image")
	if config.Server.DisableResize && isImage {
		return rawFileHandler(w, r, d.fileInfo)
	}
	if !preview.AvailablePreview(d.fileInfo.ItemInfo) {
		if isImage {
			return rawFileHandler(w, r, d.fileInfo)
		}
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", d.fileInfo.Type)
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
	previewImg, err := preview.GetPreviewForFile(d.fileInfo, previewSize, officeUrl, seekPercentage)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Cache-Control", "private")
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, d.fileInfo.Name+"-preview.jpg", d.fileInfo.ModTime, bytes.NewReader(previewImg))
	return 0, nil
}
