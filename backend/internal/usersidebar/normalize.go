package usersidebar

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
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
	skipped := []string{}
	changed := false

	for _, link := range links {
		if strings.HasPrefix(link.Category, "source") {
			source, ok := resolveSourceLink(link)
			if !ok {
				skipped = append(skipped, fmt.Sprintf(
					"{name:%q sourceName:%q reason:unresolvable}",
					link.Name, link.SourceName,
				))
				changed = true
				continue
			}
			if _, dup := seenSourcePaths[source.Path]; dup {
				skipped = append(skipped, fmt.Sprintf(
					"{name:%q sourceName:%q reason:duplicate path %q}",
					link.Name, link.SourceName, source.Path,
				))
				changed = true
				continue
			}
			seenSourcePaths[source.Path] = struct{}{}

			normalized := link
			normalized.Name = source.Name
			normalized.SourceName = source.Path
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
				skipped = append(skipped, fmt.Sprintf("{name:%q reason:duplicate tools link}", link.Name))
				changed = true
				continue
			}
			hasTools = true
		}
		out = append(out, link)
	}

	if changed {
		logger.Debugf(
			"sidebar_normalize in=%d out=%d skipped=%v out=%s",
			len(links), len(out), skipped, FormatSidebarLinksForLog(out),
		)
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
