package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"

	_ "github.com/gtsteffaniak/filebrowser/swagger/docs"
)

func publicShareHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	file, ok := d.raw.(files.ExtendedFileInfo)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to assert type *files.FileInfo")
	}
	file.Path = strings.TrimPrefix(file.Path, settings.Config.Server.Root)
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

func publicDlHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	file, ok := d.raw.(files.ExtendedFileInfo)
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("failed to assert type files.FileInfo")
	}
	if d.user == nil {
		return http.StatusUnauthorized, fmt.Errorf("failed to get user")
	}

	if file.Type == "directory" {
		return rawFilesHandler(w, r, d, []string{file.Path})
	}

	return rawFileHandler(w, r, file.FileInfo)
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
