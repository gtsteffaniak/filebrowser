package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/go-logger/logger"

	_ "github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
)

// rawHandler serves the raw content of a file, multiple files, or directory in various formats.
// @Summary Get raw content of a file, multiple files, or directory
// @Description Returns the raw content of a file, multiple files, or a directory. Supports downloading files as archives in various formats.
// @Tags Resources
// @Accept json
// @Produce json
// @Param files query string false "if specified, only the files in the list will be downloaded. eg. files=/file1||/folder/file2"
// @Param inline query bool false "If true, sets 'Content-Disposition' to 'inline'. Otherwise, defaults to 'attachment'."
// @Param algo query string false "Compression algorithm for archiving multiple files or directories. Options: 'zip' and 'tar.gz'. Default is 'zip'."
// @Success 200 {file} file "Raw file or directory content, or archive for multiple files"
// @Failure 202 {object} map[string]string "Modify permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File or directory not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /public/dl [get]
func publicRawHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("downloads are disabled for upload shares")
	}
	if d.share.DownloadsLimit > 0 && d.share.Downloads >= d.share.DownloadsLimit {
		return http.StatusForbidden, fmt.Errorf("share downloads limit reached")
	}
	d.share.Mu.Lock()
	d.share.Downloads++
	d.share.Mu.Unlock()
	encodedFiles := r.URL.Query().Get("files")

	// Decode the URL-encoded path - use PathUnescape to preserve + as literal character
	f, err := url.PathUnescape(encodedFiles)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}

	// Get the actual source name from the share's source mapping
	sourceInfo, ok := settings.Config.Server.SourceMap[d.share.Source]
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("source not found for share")
	}
	actualSourceName := sourceInfo.Name

	fileList := []string{}
	for _, file := range strings.Split(f, "||") {
		// Check if file already contains source prefix (source::path format)
		if strings.Contains(file, "::") {
			splitFile := strings.SplitN(file, "::", 2)
			if len(splitFile) == 2 {
				source := splitFile[0]
				path := splitFile[1]
				// Join the share path with the requested path
				filePath := utils.JoinPathAsUnix(d.share.Path, path)
				fileList = append(fileList, source+"::"+filePath)
			} else {
				// Fallback: treat as plain path
				filePath := utils.JoinPathAsUnix(d.share.Path, file)
				fileList = append(fileList, actualSourceName+"::"+filePath)
			}
		} else {
			// Plain path without source prefix - use the actual source name from share
			filePath := utils.JoinPathAsUnix(d.share.Path, file)
			fileList = append(fileList, actualSourceName+"::"+filePath)
		}
	}

	var status int
	status, err = rawFilesHandler(w, r, d, fileList)
	if err != nil {
		logger.Errorf("public share handler: error processing filelist: '%v' with error %v", f, err)
		return status, fmt.Errorf("error processing filelist: %v", f)
	}
	return status, nil
}

func publicShareHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if d.share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("browsing is disabled for upload shares")
	}
	return renderJSON(w, r, d.fileInfo)
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
// @Router /api/public/preview [get]
func publicPreviewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if config.Server.DisablePreviews || d.share.DisableThumbnails {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled")
	}

	if d.share.ShareType == "upload" {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled for upload shares")
	}

	// Restore source name from share for preview generation
	// The middleware clears file.Source for security, but we need it for index lookups
	if d.fileInfo.Source == "" && d.share != nil {
		sourceInfo, ok := settings.Config.Server.SourceMap[d.share.Source]
		if !ok {
			// Don't expose internal errors to share users
			return http.StatusNotFound, fmt.Errorf("resource not available")
		}
		d.fileInfo.Source = sourceInfo.Name
	}

	status, err := previewHelperFunc(w, r, d)
	if err != nil {
		// Obfuscate errors for shares to prevent information leakage
		return http.StatusNotFound, fmt.Errorf("preview not available for this item")
	}
	return status, err
}
