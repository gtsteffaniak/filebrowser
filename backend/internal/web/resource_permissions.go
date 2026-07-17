package web

import (
	"fmt"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// resourcePostPermCheck validates create vs overwrite permissions for ResourcePostHandler.
func resourcePostPermCheck(exists bool, override bool, perms users.SourceFilePermissions) (int, error) {
	if exists {
		if override && !perms.Modify {
			return http.StatusForbidden, fmt.Errorf("user is not allowed to modify")
		}
		return 0, nil
	}
	if !perms.Create {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create")
	}
	return 0, nil
}

// resourcePatchPermCheck validates move/copy/rename permissions. Returns a failure message when denied.
func resourcePatchPermCheck(action, fromSource, toSource string, fromPerms, toPerms users.SourceFilePermissions) string {
	switch action {
	case "copy":
		if !fromPerms.Download || !toPerms.Create {
			return "user is not allowed to copy"
		}
	case "move", "rename":
		if !fromPerms.Modify {
			return "user is not allowed to modify"
		}
		if toSource != fromSource && !toPerms.Modify {
			return "user is not allowed to modify destination source"
		}
	}
	return ""
}
