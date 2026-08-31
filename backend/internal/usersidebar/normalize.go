package usersidebar

import (
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// NormalizeSidebarLinks canonicalizes persisted sidebar links for storage.
// Source links are resolved via ResolveSourceKey on SourceName, with Name as fallback.
// SourceName is set to the canonical backend path; custom Name, Icon, and Category are preserved.
// Unresolvable source links are dropped. Source links are deduped by canonical path plus target
// (first occurrence wins), so multiple folder shortcuts on one source are kept. Non-source links
// pass through; duplicate Tools links are deduped.
func NormalizeSidebarLinks(links []users.SidebarLink) ([]users.SidebarLink, bool) {
	if !users.SourceConfigLoaded() {
		return links, false
	}
	if len(links) == 0 {
		return links, false
	}

	seenSourceKeys := make(map[sourceLinkDedupeKey]struct{})
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

			normalized := link
			normalized.Category = users.NormalizeSidebarLinkCategory(normalized.Category)
			normalized.SourceName = source.Path
			if strings.TrimSpace(normalized.Name) == "" {
				normalized.Name = source.Name
			}
			normalized.Target = normalizeSidebarTarget(normalized.Target)

			key := sourceLinkDedupeKey{sourcePath: source.Path, target: normalized.Target}
			if _, dup := seenSourceKeys[key]; dup {
				changed = true
				continue
			}
			seenSourceKeys[key] = struct{}{}

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

func normalizeSidebarTarget(target string) string {
	t := strings.TrimSpace(target)
	if t == "" || t == "#" {
		return "/"
	}
	if !strings.HasPrefix(t, "/") {
		t = "/" + t
	}
	if t != "/" && strings.HasSuffix(t, "/") {
		t = strings.TrimSuffix(t, "/")
	}
	return t
}

type sourceLinkDedupeKey struct {
	sourcePath string
	target     string
}
