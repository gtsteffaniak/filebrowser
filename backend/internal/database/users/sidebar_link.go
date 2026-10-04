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
	// "-root" variants display the root-filesystem-only disk usage (no nested
	// mount aggregation) instead of the aggregated multi-mount totals.
	SidebarLinkSourceRoot        SidebarLinkCategory = "source-root"
	SidebarLinkSourceAltRoot     SidebarLinkCategory = "source-alt-root"
	SidebarLinkSourceHybridRoot  SidebarLinkCategory = "source-hybrid-root"
	SidebarLinkSourceHybrid2Root SidebarLinkCategory = "source-hybrid-2-root"
	SidebarLinkTool              SidebarLinkCategory = "tool"
	SidebarLinkCustom            SidebarLinkCategory = "custom"
)

// NormalizeSidebarLinkCategory returns a known category string, preserving source-* variants.
func NormalizeSidebarLinkCategory(category string) string {
	c := strings.TrimSpace(category)
	if c == "" {
		return string(SidebarLinkSource)
	}
	switch SidebarLinkCategory(c) {
	case SidebarLinkSource, SidebarLinkSourceMinimal, SidebarLinkSourceAlt,
		SidebarLinkSourceHybrid, SidebarLinkSourceHybrid2,
		SidebarLinkSourceRoot, SidebarLinkSourceAltRoot,
		SidebarLinkSourceHybridRoot, SidebarLinkSourceHybrid2Root,
		SidebarLinkTool, SidebarLinkCustom:
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

// SidebarLinkDiskScopeRoot reports whether the category shows the
// root-filesystem-only disk usage rather than the aggregated nested-mount usage.
func SidebarLinkDiskScopeRoot(category string) bool {
	return strings.HasSuffix(NormalizeSidebarLinkCategory(category), "-root")
}

// BaseSidebarLinkCategory strips the root-only "-root" suffix from a category.
func BaseSidebarLinkCategory(category string) string {
	return strings.TrimSuffix(NormalizeSidebarLinkCategory(category), "-root")
}
