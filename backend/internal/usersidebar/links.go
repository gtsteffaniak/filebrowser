package usersidebar

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// FormatSidebarLinksForLog returns an ordered debug representation of sidebar links.
func FormatSidebarLinksForLog(links []users.SidebarLink) string {
	if len(links) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(links))
	for _, link := range links {
		parts = append(parts, fmt.Sprintf(
			"{name:%q category:%q sourceName:%q}",
			link.Name, link.Category, link.SourceName,
		))
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// FrontendLinks converts backend sidebar links to frontend-style links.
func FrontendLinks(links []users.SidebarLink, showToolsInSidebar bool) []users.SidebarLink {
	if !users.SourceConfigLoaded() {
		return []users.SidebarLink{}
	}
	hasTools := false
	newLinks := []users.SidebarLink{}
	skipped := []string{}
	for _, link := range links {
		if strings.HasPrefix(link.Category, "source") {
			if link.SourceName == "" {
				skipped = append(skipped, fmt.Sprintf("{name:%q reason:empty sourceName}", link.Name))
				continue
			}
			source, ok := users.ResolveSourceKey(link.SourceName)
			if !ok {
				skipped = append(skipped, fmt.Sprintf(
					"{name:%q sourceName:%q reason:ResolveSourceKey miss}",
					link.Name, link.SourceName,
				))
				continue
			}
			if full, ok := settings.Config.Server.SourceMap[source.Path]; ok {
				if full.Config.ResolvedRules.IndexingDisabled && link.Category != "source-minimal" {
					link.Category = "source-alt"
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
	if len(skipped) > 0 || len(links) != len(newLinks) {
		logger.Debugf(
			"sidebar_api FrontendLinks in=%d out=%d skipped=%v out=%s",
			len(links), len(newLinks), skipped, FormatSidebarLinksForLog(newLinks),
		)
	}
	return newLinks
}
