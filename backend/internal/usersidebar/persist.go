package usersidebar

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// PrepareSidebarLinksForPersist normalizes sidebar links, prunes links for revoked scopes,
// and adds missing scoped source entries. Sources absent from config are dropped by NormalizeSidebarLinks.
func PrepareSidebarLinksForPersist(links []users.SidebarLink, scopes []users.BackendScope) ([]users.SidebarLink, bool) {
	updated := false

	if normalized, changed := NormalizeSidebarLinks(links); changed {
		links = normalized
		updated = true
	}

	if pruned, changed := PruneSidebarLinksForScopes(links, scopes); changed {
		links = pruned
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
