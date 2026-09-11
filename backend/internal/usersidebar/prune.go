package usersidebar

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// PruneSidebarLinksForScopes removes source-type links whose source is not in the user's scopes.
// Non-source links (tool, custom, divider) are preserved.
func PruneSidebarLinksForScopes(links []users.SidebarLink, scopes []users.BackendScope) ([]users.SidebarLink, bool) {
	if !users.SourceConfigLoaded() || len(scopes) == 0 {
		return links, false
	}

	allowed := make(map[string]struct{}, len(scopes))
	for _, path := range uniqueScopedSourcePaths(scopes) {
		allowed[path] = struct{}{}
	}

	out := make([]users.SidebarLink, 0, len(links))
	changed := false
	for _, link := range links {
		if users.IsSourceSidebarCategory(link.Category) {
			source, ok := resolveSourceLink(link)
			if !ok {
				changed = true
				continue
			}
			if _, ok := allowed[source.Path]; !ok {
				changed = true
				continue
			}
		}
		out = append(out, link)
	}

	return out, changed
}
