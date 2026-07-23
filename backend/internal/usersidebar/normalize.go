package usersidebar

import (
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// NormalizeSidebarLinks canonicalizes persisted sidebar links for storage.
// Source links are resolved via ResolveSourceKey on SourceName, with Name as fallback.
// SourceName is set to the canonical backend path; Name to the configured display name.
// Unresolvable source links are dropped. Source links are deduped by canonical path
// (first occurrence wins). Non-source links pass through; duplicate Tools links are deduped.
func NormalizeSidebarLinks(links []users.SidebarLink) ([]users.SidebarLink, bool) {
	if !users.SourceConfigLoaded() {
		return links, false
	}
	if len(links) == 0 {
		return links, false
	}

	seenSourcePaths := make(map[string]struct{})
	hasTools := false
	out := make([]users.SidebarLink, 0, len(links))
	changed := false

	for _, link := range links {
		if users.IsSourceSidebarCategory(link.Category) {
			source, ok := resolveSourceLink(link)
			if !ok {
				changed = true
				continue
			}
			if _, dup := seenSourcePaths[source.Path]; dup {
				changed = true
				continue
			}
			seenSourcePaths[source.Path] = struct{}{}

			normalized := link
			normalized.Category = users.NormalizeSidebarLinkCategory(normalized.Category)
			normalized.SourceName = source.Path
			if strings.TrimSpace(normalized.Name) == "" {
				normalized.Name = source.Name
			}
			if normalized.Target == "" {
				normalized.Target = "/"
			}
			if normalized != link {
				changed = true
			}
			out = append(out, normalized)
			continue
		}

		if link.Category == "tool" && link.Target == "/tools" {
			if hasTools {
				changed = true
				continue
			}
			hasTools = true
		}
		out = append(out, link)
	}

	return out, changed
}

func resolveSourceLink(link users.SidebarLink) (users.SourceInfo, bool) {
	if link.SourceName != "" {
		if source, ok := users.ResolveSourceKey(link.SourceName); ok {
			return source, true
		}
	}
	if link.Name != "" {
		return users.ResolveSourceKey(link.Name)
	}
	return users.SourceInfo{}, false
}
