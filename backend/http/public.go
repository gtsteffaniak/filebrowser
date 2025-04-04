package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"

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
	file.Path = strings.TrimPrefix(file.Path, file.Source)
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
