package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

// pinnedItemPatchRequest is the JSON body for PATCH /api/users/pinnedItems.
type pinnedItemPatchRequest struct {
	Source string `json:"source" validate:"required"`
	Path   string `json:"path" validate:"required"` // scope-relative parent directory
	Name   string `json:"name" validate:"required"` // item basename within path
}

// sharePinnedItemPatchRequest is the JSON body for PATCH /public/api/share/pinnedItems.
type sharePinnedItemPatchRequest struct {
	Path string `json:"path" validate:"required"` // share-relative parent directory
	Name string `json:"name" validate:"required"` // item basename within path
}

// scopeRelativeDirToIndexPath maps a scope-relative directory to source and index paths.
func scopeRelativeDirToIndexPath(sourceName, userScope, directoryPath string) (sourcePath, indexDirPath string, err error) {
	userScope = strings.TrimRight(userScope, "/")
	source, ok := users.ResolveSourceKey(sourceName)
	if !ok {
		return "", "", fmt.Errorf("source not found: %s", sourceName)
	}
	idx := indexing.GetIndex(source.Name)
	if idx == nil {
		return "", "", fmt.Errorf("index not found for source %s", source.Name)
	}

	cleanDir, err := utils.SanitizeUserPath(directoryPath)
	if err != nil {
		return "", "", fmt.Errorf("invalid path: %w", err)
	}

	fullDirPath := utils.JoinPathAsUnix(userScope, cleanDir)
	return source.Path, idx.MakeIndexPath(fullDirPath, true), nil
}

// normalizeShareRelativeDir returns a normalized share-relative directory path key.
func normalizeShareRelativeDir(shareRelDir string) (string, error) {
	if shareRelDir == "" || shareRelDir == "/" {
		return "/", nil
	}
	cleanDir, err := utils.SanitizeUserPath(shareRelDir)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	return utils.AddTrailingSlashIfNotExists(cleanDir), nil
}

// pinnedItemAction returns the patch action query param, defaulting to "add".
func pinnedItemAction(r *http.Request) string {
	action := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("action")))
	if action == "" {
		return "add"
	}
	return action
}

// userPatchPinnedItemsHandler adds or removes a single pinned item for the authenticated user.
// @Summary Add or remove a pinned item
// @Description Patches one pinned item at a time. Defaults to add; pass ?action=remove to unpin.
// @Tags Users
// @Accept json
// @Produce json
// @Param action query string false "add (default) or remove"
// @Param body body pinnedItemPatchRequest true "Pinned item"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string "Bad Request"
// @Failure 403 {object} map[string]string "Forbidden"
// @Failure 500 {object} map[string]string "Internal Server Error"
// @Router /api/users/pinnedItems [patch]
func userPatchPinnedItemsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var body pinnedItemPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
	}
	defer r.Body.Close()

	action := pinnedItemAction(r)
	if action != "add" && action != "remove" {
		return http.StatusBadRequest, fmt.Errorf("action must be add or remove")
	}

	if body.Source == "" || body.Path == "" || body.Name == "" {
		return http.StatusBadRequest, fmt.Errorf("source, path, and name are required")
	}
	cleanPath, err := utils.SanitizeUserPath(body.Path)
	if err != nil {
		return http.StatusBadRequest, err
	}
	body.Path = cleanPath

	cleanName, err := utils.SanitizeUserPath(body.Name)
	if err != nil {
		return http.StatusBadRequest, err
	}
	body.Name = cleanName

	source, ok := users.ResolveSourceKey(body.Source)
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("source not found: %s", body.Source)
	}
	if !d.user.HasSourceByPath(source.Path) {
		return http.StatusForbidden, fmt.Errorf("access denied to source %s", body.Source)
	}

	userScope, err := d.user.GetScopeForSourcePath(source.Path)
	if err != nil {
		return http.StatusForbidden, err
	}

	sourcePath, indexDirPath, err := scopeRelativeDirToIndexPath(body.Source, userScope, body.Path)
	if err != nil {
		return http.StatusBadRequest, err
	}

	u, err := store.Users.Get(d.user.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	pinned := u.EnsurePinnedItems()
	switch action {
	case "add":
		pinned.Add(sourcePath, indexDirPath, body.Name)
	case "remove":
		pinned.Remove(sourcePath, indexDirPath, body.Name)
	}

	if err := store.Users.Update(u, d.user.Permissions.Admin, "PinnedItems"); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}
