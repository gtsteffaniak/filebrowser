package http

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/preview"

	_ "github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
)

// rawHandler serves the raw content of a file, multiple files, or directory in various formats.
// @Summary Get raw content of a file, multiple files, or directory
// @Description Returns the raw content of a file, multiple files, or a directory. Supports downloading files as archives in various formats.
// @Tags Resources
// @Accept json
// @Produce json
// @Param files query string true "a list of files in the following format 'filename' and separated by '||' with additional items in the list. (required)"
// @Param inline query bool false "If true, sets 'Content-Disposition' to 'inline'. Otherwise, defaults to 'attachment'."
// @Param algo query string false "Compression algorithm for archiving multiple files or directories. Options: 'zip' and 'tar.gz'. Default is 'zip'."
// @Success 200 {file} file "Raw file or directory content, or archive for multiple files"
// @Failure 202 {object} map[string]string "Modify permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File or directory not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /public/dl [get]
func publicRawHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	encodedFiles := r.URL.Query().Get("files")
	// Decode the URL-encoded path
	f, err := url.QueryUnescape(encodedFiles)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}

	fileInfo, ok := d.raw.(iteminfo.ExtendedFileInfo)
	if !ok {
		logger.Error("public share handler: failed to assert type files.FileInfo")
		return http.StatusInternalServerError, fmt.Errorf("failed to assert type files.FileInfo")
	}
	fileList := []string{}
	for _, file := range strings.Split(f, "||") {
		fileList = append(fileList, fileInfo.Source+"::"+fileInfo.Path+file)
	}
	var status int
	status, err = rawFilesHandler(w, r, d, fileList)
	if err != nil {
		logger.Error(fmt.Sprintf("public share handler: error processing filelist: %v", err))
		return status, fmt.Errorf("error processing filelist: %v", f)
	}
	return status, nil
}

func publicShareHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	file, ok := d.raw.(iteminfo.ExtendedFileInfo)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to assert type iteminfo.FileInfo")
	}
	file.Path = strings.TrimPrefix(file.Path, d.share.Path)
	return renderJSON(w, r, file)
}

func publicUserGetHandler(w http.ResponseWriter, r *http.Request) {
	// Call the actual handler logic here (e.g., renderJSON, etc.)
	// You may need to replace `fn` with the actual handler logic.
	status, err := renderJSON(w, r, users.PublicUser)
	if err != nil {
		http.Error(w, http.StatusText(status), status)
	}
}

// health godoc
// @Summary Health Check
// @Schemes
// @Description Returns the health status of the API.
// @Tags Health
// @Accept json
// @Produce json
// @Success 200 {object} HttpResponse "successful health check response"
// @Router /health [get]
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := HttpResponse{Message: "ok"}    // Create response with status "ok"
	err := json.NewEncoder(w).Encode(response) // Encode the response into JSON
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// publicPreviewHandler handles the preview request for images from shares.
// @Summary Get image preview
// @Description Returns a preview image based on the requested path and size.
// @Tags Resources
// @Accept json
// @Produce json
// @Param hash query string true "source hash"
// @Param path query string true "File path of the image to preview"
// @Param size query string false "Preview size ('small' or 'large'). Default is based on server config."
// @Success 200 {file} file "Preview image content"
// @Failure 202 {object} map[string]string "Download permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 415 {object} map[string]string "Unsupported file type for preview"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/preview [get]
func publicPreviewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
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
	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Path:   utils.JoinPathAsUnix(d.share.Path, path),
		Modify: d.user.Permissions.Modify,
		Source: source,
		Expand: true,
	})
	if err != nil {
		logger.Debug(fmt.Sprintf("public preview handler: error getting file info: %v", err))
		return 400, fmt.Errorf("file not found")
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
	if err == preview.ErrUnsupportedFormat {
		return rawFileHandler(w, r, fileInfo)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, fileInfo.RealPath, fileInfo.ModTime, bytes.NewReader(previewImg))
	return 0, nil
}
