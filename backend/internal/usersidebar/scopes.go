package usersidebar

import (
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// EnsureSidebarLinksFromScopes adds source sidebar links for scoped sources that have no link yet.
// Existing links (including custom names, icons, and categories) are preserved.
func EnsureSidebarLinksFromScopes(links []users.SidebarLink, scopes []users.BackendScope) ([]users.SidebarLink, bool) {
	if !users.SourceConfigLoaded() || len(scopes) == 0 {
		return links, false
	}

	present := sourcePathsInSidebarLinks(links)
	out := append([]users.SidebarLink(nil), links...)
	changed := false

	for _, scope := range uniqueScopedSourcePaths(scopes) {
		if _, ok := present[scope]; ok {
			continue
		}
		source, ok := users.ResolveSourceKey(scope)
		if !ok {
			continue
		}
		out = append(out, users.SidebarLink{
			Name:       source.Name,
			Category:   string(users.SidebarLinkSource),
			Target:     "/",
			SourceName: source.Path,
		})
		present[source.Path] = struct{}{}
		changed = true
	}

	return out, changed
}

// ValidSourceSidebarLinkCount returns how many source links resolve to a configured source.
func ValidSourceSidebarLinkCount(links []users.SidebarLink) int {
	count := 0
	for _, link := range links {
		if !users.IsSourceSidebarCategory(link.Category) {
			continue
		}
		if _, ok := resolveSourceLink(link); ok {
			count++
		}
	}
	return count
}

func uniqueScopedSourcePaths(scopes []users.BackendScope) []string {
	seen := make(map[string]struct{}, len(scopes))
	out := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		path := strings.TrimSpace(scope.Path)
		if path == "" {
			continue
		}
		if _, dup := seen[path]; dup {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func sourcePathsInSidebarLinks(links []users.SidebarLink) map[string]struct{} {
	present := make(map[string]struct{})
	for _, link := range links {
		if !users.IsSourceSidebarCategory(link.Category) {
			continue
		}
		source, ok := resolveSourceLink(link)
		if !ok {
			continue
		}
		present[source.Path] = struct{}{}
	}
	return present
}

func NeedsSidebarLinksFromScopes(links []users.SidebarLink, scopes []users.BackendScope) bool {
	scopePaths := uniqueScopedSourcePaths(scopes)
	if len(scopePaths) == 0 {
		return false
	}
	valid := ValidSourceSidebarLinkCount(links)
	if valid == 0 {
		return true
	}
	present := sourcePathsInSidebarLinks(links)
	for _, path := range scopePaths {
		if _, ok := present[path]; !ok {
			return true
		}
	}
	return false
}
