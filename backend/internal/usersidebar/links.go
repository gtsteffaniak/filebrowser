package usersidebar

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// prepareFrontendLink converts a stored sidebar link for API responses (source paths → display names).
func prepareFrontendLink(link users.SidebarLink) (users.SidebarLink, bool) {
	if !users.IsSourceSidebarCategory(link.Category) {
		return link, true
	}
	source, ok := resolveSourceLink(link)
	if !ok {
		return link, false
	}
	if full, ok := settings.Config.Server.SourceMap[source.Path]; ok {
		category := users.NormalizeSidebarLinkCategory(link.Category)
		if full.Config.ResolvedRules.IndexingDisabled && category == string(users.SidebarLinkSource) {
			link.Category = string(users.SidebarLinkSourceAlt)
		}
	}
	link.SourceName = source.Name
	return link, true
}

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
			prepared, ok := prepareFrontendLink(link)
			if !ok {
				continue
			}
			link = prepared
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
