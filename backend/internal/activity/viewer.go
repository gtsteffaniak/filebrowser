package activity

import (
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// PrepareItemsForViewer trims activity paths for non-admin viewers.
func PrepareItemsForViewer(items []activitydb.FrontendEntry, viewer *users.User) {
	if viewer == nil || viewer.Permissions.Admin {
		return
	}
	for i := range items {
		items[i].TrimPathsForUser(viewer)
	}
}
