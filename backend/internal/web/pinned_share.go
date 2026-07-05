package web

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

type sharePinnedItemPatchRequest struct {
	Path string `json:"path" validate:"required"`
	Name string `json:"name" validate:"required"`
}

func normalizeShareRelativeDir(shareRelDir string) (string, error) {
	if shareRelDir == "" || shareRelDir == "/" {
		return "/", nil
	}
	cleanDir, err := utils.SanitizePath(shareRelDir)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	return utils.AddTrailingSlashIfNotExists(cleanDir), nil
}

// sharePatchPinnedItemsHandler adds or removes a pinned item on a share.
func sharePatchPinnedItemsHandler(w http.ResponseWriter, r *http.Request, d *Context) (int, error) {
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		return http.StatusBadRequest, fmt.Errorf("hash is required")
	}

	link, err := state.GetShare(hash)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("share hash not found")
	}

	if d.User.Username == "anonymous" || !link.UserCanEdit(d.User) {
		return http.StatusForbidden, fmt.Errorf("share pin editing is not allowed for this user")
	}

	if link.ShareType == "upload" {
		return http.StatusForbidden, fmt.Errorf("pinning is disabled for upload shares")
	}

	var body sharePinnedItemPatchRequest
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
	}
	defer r.Body.Close()

	action := pinnedItemAction(r)
	if action != "add" && action != "remove" {
		return http.StatusBadRequest, fmt.Errorf("action must be add or remove")
	}

	if body.Path == "" || body.Name == "" {
		return http.StatusBadRequest, fmt.Errorf("path and name are required")
	}

	shareRelDir, err := normalizeShareRelativeDir(body.Path)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path: %s", body.Path)
	}

	cleanName, err := utils.SanitizePath(body.Name)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid name: %s", body.Name)
	}
	body.Name = cleanName

	pinned := link.EnsurePinnedItems()
	switch action {
	case "add":
		pinned.Add(shareRelDir, body.Name)
	case "remove":
		pinned.Remove(shareRelDir, body.Name)
	}

	if err := state.UpdateShare(hash, func(existing *share.Share) error {
		existing.PinnedItems = link.PinnedItems
		return nil
	}); err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to save share: %w", err)
	}

	return http.StatusNoContent, nil
}
