package usersidebar

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// ErrEnforcedSidebarLinkRemoved is returned when a non-admin removes or alters an enforced link.
type ErrEnforcedSidebarLinkRemoved struct {
	Key string
}

func (e ErrEnforcedSidebarLinkRemoved) Error() string {
	return fmt.Sprintf("sidebar link %q is enforced by an administrator", e.Key)
}

// LinkKey returns a stable identity for merge, dedup, and enforcement checks (persisted/path form).
func LinkKey(link users.SidebarLink) string {
	if users.IsSourceSidebarCategory(link.Category) {
		if source, ok := resolveSourceLink(link); ok {
			target := normalizeSidebarTarget(link.Target)
			return fmt.Sprintf("source:%s:%s", source.Path, target)
		}
	}
	target := strings.TrimSpace(link.Target)
	if link.Category == "tool" && target == "/tools" {
		return "tool:/tools"
	}
	return fmt.Sprintf("%s:%s", strings.TrimSpace(link.Category), target)
}

// LinkKeyForDisplay returns a stable key using source display names for API responses.
func LinkKeyForDisplay(link users.SidebarLink) string {
	if users.IsSourceSidebarCategory(link.Category) {
		if source, ok := resolveSourceLink(link); ok {
			target := normalizeSidebarTarget(link.Target)
			return fmt.Sprintf("source:%s:%s", source.Name, target)
		}
	}
	return LinkKey(link)
}

// UserHasSourceAccess reports whether the user has a backend scope on the given source path.
func UserHasSourceAccess(scopes []users.BackendScope, sourcePath string) bool {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return false
	}
	for _, scope := range scopes {
		if scope.Path == sourcePath {
			return true
		}
	}
	return false
}

func defaultItemAppliesToUser(item SidebarLinkDefaultItem, scopes []users.BackendScope) bool {
	if !users.IsSourceSidebarCategory(item.Link.Category) {
		return true
	}
	source, ok := resolveSourceLink(item.Link)
	if !ok {
		return false
	}
	return UserHasSourceAccess(scopes, source.Path)
}

func prepareDefaultLink(link users.SidebarLink) (users.SidebarLink, bool) {
	normalized, changed := NormalizeSidebarLinks([]users.SidebarLink{link})
	if len(normalized) == 0 {
		return users.SidebarLink{}, false
	}
	_ = changed
	return normalized[0], true
}

func linkKeysPresent(links []users.SidebarLink) map[string]users.SidebarLink {
	out := make(map[string]users.SidebarLink, len(links))
	for _, link := range links {
		out[LinkKey(link)] = link
	}
	return out
}

func linksEquivalent(a, b users.SidebarLink) bool {
	na, okA := prepareDefaultLink(a)
	nb, okB := prepareDefaultLink(b)
	if !okA || !okB {
		return false
	}
	return na.Name == nb.Name &&
		na.Category == nb.Category &&
		na.Target == nb.Target &&
		na.Icon == nb.Icon &&
		na.SourceName == nb.SourceName
}

// MergeDefaultLinks appends enabled default items missing from links. Returns updated links and whether changed.
func MergeDefaultLinks(links []users.SidebarLink, scopes []users.BackendScope, doc SidebarLinkDefaultsDocument) ([]users.SidebarLink, bool) {
	present := linkKeysPresent(links)
	out := append([]users.SidebarLink(nil), links...)
	changed := false

	for _, item := range doc.Items {
		if !item.Enabled {
			continue
		}
		if !defaultItemAppliesToUser(item, scopes) {
			continue
		}
		prepared, ok := prepareDefaultLink(item.Link)
		if !ok {
			continue
		}
		key := LinkKey(prepared)
		if _, exists := present[key]; exists {
			continue
		}
		out = append(out, prepared)
		present[key] = prepared
		changed = true
	}

	return out, changed
}

// MergeEnforcedLinks ensures enforced items are present and match the admin template for non-admins.
func MergeEnforcedLinks(links []users.SidebarLink, scopes []users.BackendScope, doc SidebarLinkDefaultsDocument, isAdmin bool) ([]users.SidebarLink, bool) {
	if isAdmin {
		return links, false
	}
	present := linkKeysPresent(links)
	out := append([]users.SidebarLink(nil), links...)
	changed := false

	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if !defaultItemAppliesToUser(item, scopes) {
			continue
		}
		prepared, ok := prepareDefaultLink(item.Link)
		if !ok {
			continue
		}
		key := LinkKey(prepared)
		if existing, exists := present[key]; exists {
			if !linksEquivalent(existing, prepared) {
				for i := range out {
					if LinkKey(out[i]) == key {
						out[i] = prepared
						changed = true
						break
					}
				}
				present[key] = prepared
			}
			continue
		}
		out = append(out, prepared)
		present[key] = prepared
		changed = true
	}

	return out, changed
}

// ValidateEnforcedSidebarLinks returns an error if a non-admin removed or altered an enforced link.
func ValidateEnforcedSidebarLinks(links []users.SidebarLink, scopes []users.BackendScope, doc SidebarLinkDefaultsDocument, isAdmin bool) error {
	if isAdmin {
		return nil
	}
	present := linkKeysPresent(links)
	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if !defaultItemAppliesToUser(item, scopes) {
			continue
		}
		prepared, ok := prepareDefaultLink(item.Link)
		if !ok {
			continue
		}
		key := LinkKey(prepared)
		existing, exists := present[key]
		if !exists {
			return ErrEnforcedSidebarLinkRemoved{Key: key}
		}
		if !linksEquivalent(existing, prepared) {
			return ErrEnforcedSidebarLinkRemoved{Key: key}
		}
	}
	return nil
}

// EnforcedLinkKeys returns stable keys for enforced items that apply to the given user.
func EnforcedLinkKeys(scopes []users.BackendScope, doc SidebarLinkDefaultsDocument, isAdmin bool) []string {
	if isAdmin {
		return nil
	}
	keys := make([]string, 0)
	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if !defaultItemAppliesToUser(item, scopes) {
			continue
		}
		prepared, ok := prepareDefaultLink(item.Link)
		if !ok {
			continue
		}
		keys = append(keys, LinkKeyForDisplay(prepared))
	}
	return keys
}

// DocumentWithAllSources merges configured sources into the defaults document for admin UI.
func DocumentWithAllSources(doc SidebarLinkDefaultsDocument) SidebarLinkDefaultsDocument {
	if !users.SourceConfigLoaded() {
		return doc
	}
	bySource := make(map[string]SidebarLinkDefaultItem)
	custom := make([]SidebarLinkDefaultItem, 0, len(doc.Items))
	for _, item := range doc.Items {
		if users.IsSourceSidebarCategory(item.Link.Category) {
			if source, ok := resolveSourceLink(item.Link); ok {
				bySource[source.Path] = item
				continue
			}
		}
		custom = append(custom, item)
	}

	out := make([]SidebarLinkDefaultItem, 0, len(settings.Config.Server.Sources)+len(custom))
	for _, src := range settings.Config.Server.Sources {
		if src == nil {
			continue
		}
		if item, ok := bySource[src.Path]; ok {
			out = append(out, item)
			continue
		}
		out = append(out, SidebarLinkDefaultItem{
			Enabled: true,
			Link: users.SidebarLink{
				Name:       src.Name,
				Category:   string(users.SidebarLinkSource),
				Target:     "/",
				SourceName: src.Path,
			},
		})
	}
	out = append(out, custom...)
	return SidebarLinkDefaultsDocument{Items: out}
}

// ConfiguredSourceNames returns display names for all configured sources.
func ConfiguredSourceNames() []string {
	if !users.SourceConfigLoaded() {
		return nil
	}
	names := make([]string, 0, len(settings.Config.Server.Sources))
	for _, src := range settings.Config.Server.Sources {
		if src == nil {
			continue
		}
		names = append(names, src.Name)
	}
	return names
}

// EnsureAllSourcesInDefaults merges in any configured sources missing from doc (enabled by default).
func EnsureAllSourcesInDefaults(doc SidebarLinkDefaultsDocument) (SidebarLinkDefaultsDocument, bool) {
	merged := DocumentWithAllSources(doc)
	if len(merged.Items) == len(doc.Items) {
		return doc, false
	}
	return merged, true
}

// InitialSidebarLinkDefaultsDocument returns enabled defaults for every configured source.
func InitialSidebarLinkDefaultsDocument() SidebarLinkDefaultsDocument {
	return DocumentWithAllSources(SidebarLinkDefaultsDocument{Items: []SidebarLinkDefaultItem{}})
}

// FrontendDefaultItem converts one defaults item for API responses.
func FrontendDefaultItem(item SidebarLinkDefaultItem) SidebarLinkDefaultItem {
	if prepared, ok := prepareFrontendLink(item.Link); ok {
		item.Link = prepared
	}
	return item
}

// FrontendDefaultsDocument converts stored defaults for API responses.
func FrontendDefaultsDocument(doc SidebarLinkDefaultsDocument) SidebarLinkDefaultsDocument {
	if len(doc.Items) == 0 {
		return doc
	}
	out := make([]SidebarLinkDefaultItem, len(doc.Items))
	for i, item := range doc.Items {
		out[i] = FrontendDefaultItem(item)
	}
	return SidebarLinkDefaultsDocument{Items: out}
}

// NormalizeDefaultsDocument canonicalizes incoming defaults for storage.
func NormalizeDefaultsDocument(doc SidebarLinkDefaultsDocument) SidebarLinkDefaultsDocument {
	if len(doc.Items) == 0 {
		return doc
	}
	out := make([]SidebarLinkDefaultItem, 0, len(doc.Items))
	for _, item := range doc.Items {
		normalized, _ := NormalizeSidebarLinks([]users.SidebarLink{item.Link})
		if len(normalized) == 0 {
			continue
		}
		out = append(out, SidebarLinkDefaultItem{
			Enabled:  item.Enabled,
			Enforced: item.Enforced,
			Link:     normalized[0],
		})
	}
	return SidebarLinkDefaultsDocument{Items: out}
}
