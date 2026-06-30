package http

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// internalDeleteUserHandler handles DELETE /api/internal/delete-user?email=
// Called by the landing page during account deletion. Authenticated via x-api-key header.
func internalDeleteUserHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Fail closed if no secret is configured, and require a sufficiently long key. The
	// comparison is constant-time to avoid a timing side-channel on the shared secret.
	secret := os.Getenv("ACORN_DRIVE_API_SECRET")
	provided := r.Header.Get("x-api-key")
	if len(secret) < 16 || subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
		logger.Warningf("[internal-delete] unauthorized delete attempt from %s", r.RemoteAddr)
		return http.StatusUnauthorized, fmt.Errorf("unauthorized")
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		return http.StatusBadRequest, fmt.Errorf("email is required")
	}

	// Username == email in this system (set from Azure JWT preferred_username)
	user, err := store.Users.Get(email)
	if err != nil || user == nil {
		return http.StatusNotFound, fmt.Errorf("user not found: %s", email)
	}

	// Delete each home directory the user owns
	for _, scope := range user.Scopes {
		source, ok := settings.Config.Server.SourceMap[scope.Name]
		if !ok {
			logger.Errorf("[internal-delete] source not found for scope %s, skipping", scope.Name)
			continue
		}
		fullPath := filepath.Join(source.Path, scope.Scope)
		if removeErr := os.RemoveAll(fullPath); removeErr != nil {
			logger.Errorf("[internal-delete] failed to remove directory %s: %v", fullPath, removeErr)
		}
	}

	// Delete the user record — access rules and share links become inert without the files
	if err := store.Users.Delete(user.ID); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to delete user record: %w", err)
	}

	return http.StatusOK, nil
}
