package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/share"
	"github.com/gtsteffaniak/filebrowser/users"

	_ "github.com/gtsteffaniak/filebrowser/swagger/docs"
)

func publicShareHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	file, ok := d.raw.(files.FileInfo)
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
	file, _ := d.raw.(*files.FileInfo)
	if file == nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to assert type files.FileInfo")
	}
	if d.user == nil {
		return http.StatusUnauthorized, fmt.Errorf("failed to get user")
	}

	if file.Type == "directory" {
		return rawDirHandler(w, r, d, file)
	}

	return rawFileHandler(w, r, file)
}

func authenticateShareRequest(r *http.Request, l *share.Link) (int, error) {
	if l.PasswordHash == "" {
		return 200, nil
	}

	if r.URL.Query().Get("token") == l.Token {
		return 200, nil
	}

	password := r.Header.Get("X-SHARE-PASSWORD")
	password, err := url.QueryUnescape(password)
	if err != nil {
		return http.StatusUnauthorized, err
	}
	if password == "" {
		return http.StatusUnauthorized, nil
	}
	if err := bcrypt.CompareHashAndPassword([]byte(l.PasswordHash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return http.StatusUnauthorized, nil
		}
		return 401, err
	}
	return 200, nil
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
