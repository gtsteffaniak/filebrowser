package usersidebar

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// PrepareSidebarLinksForPersist normalizes sidebar links and adds missing scoped source entries.
// Inaccessible sources are not pruned; callers rely on the UI to gray out stale links.
func PrepareSidebarLinksForPersist(links []users.SidebarLink, scopes []users.BackendScope) ([]users.SidebarLink, bool) {
	updated := false

	if normalized, changed := NormalizeSidebarLinks(links); changed {
		links = normalized
		updated = true
	}

	merged, changed := EnsureSidebarLinksFromScopes(links, scopes)
	if changed {
		links = merged
		updated = true
		if normalized, changed := NormalizeSidebarLinks(links); changed {
			links = normalized
			updated = true
		}
	}

	return links, updated
}
