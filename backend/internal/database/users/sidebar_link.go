package users

import "strings"

// SidebarLinkCategory identifies sidebar link presentation types.
type SidebarLinkCategory string

const (
	SidebarLinkSource        SidebarLinkCategory = "source"
	SidebarLinkSourceMinimal SidebarLinkCategory = "source-minimal"
	SidebarLinkSourceAlt     SidebarLinkCategory = "source-alt"
	SidebarLinkSourceHybrid  SidebarLinkCategory = "source-hybrid"
	SidebarLinkSourceHybrid2 SidebarLinkCategory = "source-hybrid-2"
	SidebarLinkTool          SidebarLinkCategory = "tool"
	SidebarLinkCustom        SidebarLinkCategory = "custom"
)

// NormalizeSidebarLinkCategory returns a known category string, preserving source-* variants.
func NormalizeSidebarLinkCategory(category string) string {
	c := strings.TrimSpace(category)
	if c == "" {
		return string(SidebarLinkSource)
	}
	switch SidebarLinkCategory(c) {
	case SidebarLinkSource, SidebarLinkSourceMinimal, SidebarLinkSourceAlt,
		SidebarLinkSourceHybrid, SidebarLinkSourceHybrid2, SidebarLinkTool, SidebarLinkCustom:
		return c
	}
	if strings.HasPrefix(c, "source") {
		return c
	}
	return c
}

// IsSourceSidebarCategory reports whether the category is a source-style sidebar link.
func IsSourceSidebarCategory(category string) bool {
	return strings.HasPrefix(NormalizeSidebarLinkCategory(category), "source")
}
