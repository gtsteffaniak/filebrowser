package toolaccess

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// ErrEnforcedToolAccess is returned when a non-admin changes an enforced tool access value.
type ErrEnforcedToolAccess struct {
	ToolID users.ToolID
}

func (e ErrEnforcedToolAccess) Error() string {
	return fmt.Sprintf("tool access for %q is enforced by an administrator", e.ToolID)
}

// InitialToolAccessDefaultsDocument seeds all catalog tools as enabled and not enforced.
func InitialToolAccessDefaultsDocument() ToolAccessDefaultsDocument {
	items := make([]ToolAccessDefaultItem, 0, len(users.AllToolIDs))
	for _, id := range users.AllToolIDs {
		items = append(items, ToolAccessDefaultItem{
			ToolID:   id,
			Enabled:  true,
			Enforced: false,
		})
	}
	return ToolAccessDefaultsDocument{Items: items}
}

// EnsureCatalogInDefaults adds any catalog tools missing from the document.
func EnsureCatalogInDefaults(doc ToolAccessDefaultsDocument) (ToolAccessDefaultsDocument, bool) {
	if doc.Items == nil {
		doc.Items = []ToolAccessDefaultItem{}
	}
	present := make(map[users.ToolID]bool, len(doc.Items))
	for _, item := range doc.Items {
		if item.ToolID.Valid() {
			present[item.ToolID] = true
		}
	}
	changed := false
	for _, id := range users.AllToolIDs {
		if present[id] {
			continue
		}
		doc.Items = append(doc.Items, ToolAccessDefaultItem{
			ToolID:   id,
			Enabled:  true,
			Enforced: false,
		})
		changed = true
	}
	return doc, changed
}

// NormalizeDefaultsDocument keeps only known tools and deduplicates by tool ID (last wins).
func NormalizeDefaultsDocument(doc ToolAccessDefaultsDocument) ToolAccessDefaultsDocument {
	if doc.Items == nil {
		doc.Items = []ToolAccessDefaultItem{}
	}
	byID := make(map[users.ToolID]ToolAccessDefaultItem, len(doc.Items))
	for _, item := range doc.Items {
		if !item.ToolID.Valid() {
			continue
		}
		byID[item.ToolID] = ToolAccessDefaultItem{
			ToolID:   item.ToolID,
			Enabled:  item.Enabled,
			Enforced: item.Enforced,
		}
	}
	out := make([]ToolAccessDefaultItem, 0, len(users.AllToolIDs))
	for _, id := range users.AllToolIDs {
		if item, ok := byID[id]; ok {
			out = append(out, item)
		}
	}
	return ToolAccessDefaultsDocument{Items: out}
}

func defaultItemFor(doc ToolAccessDefaultsDocument, toolID users.ToolID) *ToolAccessDefaultItem {
	for i := range doc.Items {
		if doc.Items[i].ToolID == toolID {
			return &doc.Items[i]
		}
	}
	return nil
}

func userToolAccess(u *users.User, toolID users.ToolID) (bool, bool) {
	if u == nil || u.ToolAccess == nil {
		return false, false
	}
	v, ok := u.ToolAccess[toolID]
	return v, ok
}

// HasToolAccess reports effective access for u to toolID using doc defaults.
func HasToolAccess(u *users.User, toolID users.ToolID, doc ToolAccessDefaultsDocument) bool {
	if u != nil && u.Permissions.Admin {
		return true
	}
	if !toolID.Valid() {
		return false
	}
	item := defaultItemFor(doc, toolID)
	if item != nil && item.Enforced {
		return item.Enabled
	}
	if v, ok := userToolAccess(u, toolID); ok {
		return v
	}
	if item != nil {
		return item.Enabled
	}
	return true
}

// EffectiveToolAccessMap returns effective access for every catalog tool (string keys for API).
func EffectiveToolAccessMap(u *users.User, doc ToolAccessDefaultsDocument) map[string]bool {
	out := make(map[string]bool, len(users.AllToolIDs))
	for _, id := range users.AllToolIDs {
		out[string(id)] = HasToolAccess(u, id, doc)
	}
	return out
}

// HasAnyToolAccess reports whether u has access to at least one of the tool IDs.
func HasAnyToolAccess(u *users.User, toolIDs []users.ToolID, doc ToolAccessDefaultsDocument) bool {
	for _, id := range toolIDs {
		if HasToolAccess(u, id, doc) {
			return true
		}
	}
	return false
}

// MergeDefaultToolAccess applies default enabled values for tools missing from u.ToolAccess.
func MergeDefaultToolAccess(u *users.User, doc ToolAccessDefaultsDocument) bool {
	if u == nil {
		return false
	}
	changed := false
	if u.ToolAccess == nil {
		u.ToolAccess = make(users.ToolAccessMap)
	}
	for _, item := range doc.Items {
		if _, exists := u.ToolAccess[item.ToolID]; exists {
			continue
		}
		u.ToolAccess[item.ToolID] = item.Enabled
		changed = true
	}
	return changed
}

// MergeEnforcedToolAccess overwrites enforced tool values for non-admin users.
func MergeEnforcedToolAccess(u *users.User, doc ToolAccessDefaultsDocument) bool {
	if u == nil || u.Permissions.Admin {
		return false
	}
	changed := false
	if u.ToolAccess == nil {
		u.ToolAccess = make(users.ToolAccessMap)
	}
	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if u.ToolAccess[item.ToolID] != item.Enabled {
			u.ToolAccess[item.ToolID] = item.Enabled
			changed = true
		}
	}
	return changed
}

// ValidateEnforcedToolAccess returns an error if a non-admin changed an enforced tool value.
func ValidateEnforcedToolAccess(u *users.User, doc ToolAccessDefaultsDocument) error {
	if u == nil || u.Permissions.Admin {
		return nil
	}
	for _, item := range doc.Items {
		if !item.Enforced {
			continue
		}
		if u.ToolAccess == nil {
			return ErrEnforcedToolAccess{ToolID: item.ToolID}
		}
		if u.ToolAccess[item.ToolID] != item.Enabled {
			return ErrEnforcedToolAccess{ToolID: item.ToolID}
		}
	}
	return nil
}
