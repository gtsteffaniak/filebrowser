package usersidebar

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// FrontendLinks converts backend sidebar links to frontend-style links.
func FrontendLinks(links []users.SidebarLink, showToolsInSidebar bool) []users.SidebarLink {
	if !users.SourceConfigLoaded() {
		return []users.SidebarLink{}
	}
	hasTools := false
	newLinks := []users.SidebarLink{}
	for _, link := range links {
		if users.IsSourceSidebarCategory(link.Category) {
			if link.SourceName == "" {
				continue
			}
			source, ok := users.ResolveSourceKey(link.SourceName)
			if !ok {
				continue
			}
			if full, ok := settings.Config.Server.SourceMap[source.Path]; ok {
				category := users.NormalizeSidebarLinkCategory(link.Category)
				if full.Config.ResolvedRules.IndexingDisabled && category == string(users.SidebarLinkSource) {
					link.Category = string(users.SidebarLinkSourceAlt)
				}
			}
			link.SourceName = source.Name
		} else if link.Category == "tool" && link.Target == "/tools" {
			hasTools = true
		}
		newLinks = append(newLinks, link)
	}
	if !hasTools && showToolsInSidebar {
		newLinks = append(newLinks, users.SidebarLink{
			Name:     "Tools",
			Category: "tool",
			Target:   "/tools",
			Icon:     "build",
		})
	}
	return newLinks
}
