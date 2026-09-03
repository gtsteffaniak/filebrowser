package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
)

const sidebarLinkDefaultsSettingKey = "sidebarLinkDefaults"

var (
	sidebarLinkDefaultsMu       sync.RWMutex
	sidebarLinkDefaults         usersidebar.SidebarLinkDefaultsDocument
	sidebarLinkDefaultsNeedResync bool
)

// InitSidebarLinkDefaults loads persisted sidebar link defaults into memory.
func InitSidebarLinkDefaults() error {
	if sqlDb == nil {
		return fmt.Errorf("sqlDb not initialized")
	}
	doc, found, err := loadSidebarLinkDefaultsDocument()
	if err != nil {
		return err
	}
	if !found {
		doc = usersidebar.InitialSidebarLinkDefaultsDocument()
		if saveErr := saveSidebarLinkDefaultsDocument(doc); saveErr != nil {
			return saveErr
		}
	} else {
		ensured, changed := usersidebar.EnsureAllSourcesInDefaults(doc)
		if changed {
			if saveErr := saveSidebarLinkDefaultsDocument(ensured); saveErr != nil {
				return saveErr
			}
			doc = ensured
			sidebarLinkDefaultsNeedResync = true
		}
	}
	sidebarLinkDefaultsMu.Lock()
	sidebarLinkDefaults = doc
	sidebarLinkDefaultsMu.Unlock()
	if !found {
		sidebarLinkDefaultsNeedResync = true
	}
	return nil
}

func loadSidebarLinkDefaultsDocument() (usersidebar.SidebarLinkDefaultsDocument, bool, error) {
	raw, err := sqlDb.GetSetting(sidebarLinkDefaultsSettingKey)
	if err != nil {
		if err.Error() == fmt.Sprintf("setting not found: %s", sidebarLinkDefaultsSettingKey) {
			return usersidebar.SidebarLinkDefaultsDocument{}, false, nil
		}
		return usersidebar.SidebarLinkDefaultsDocument{}, false, err
	}
	var doc usersidebar.SidebarLinkDefaultsDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return usersidebar.SidebarLinkDefaultsDocument{}, false, fmt.Errorf("parse %s: %w", sidebarLinkDefaultsSettingKey, err)
	}
	if doc.Items == nil {
		doc.Items = []usersidebar.SidebarLinkDefaultItem{}
	}
	return doc, true, nil
}

func saveSidebarLinkDefaultsDocument(doc usersidebar.SidebarLinkDefaultsDocument) error {
	if doc.Items == nil {
		doc.Items = []usersidebar.SidebarLinkDefaultItem{}
	}
	return sqlDb.SaveSetting(sidebarLinkDefaultsSettingKey, doc)
}

// EffectiveSidebarLinkDefaults returns the in-memory sidebar link defaults document.
func EffectiveSidebarLinkDefaults() usersidebar.SidebarLinkDefaultsDocument {
	sidebarLinkDefaultsMu.RLock()
	defer sidebarLinkDefaultsMu.RUnlock()
	return sidebarLinkDefaults
}

// SidebarLinkDefaultsSettings is the admin API response for GET /api/settings/sidebar-link-defaults.
type SidebarLinkDefaultsSettings struct {
	Items   []usersidebar.SidebarLinkDefaultItem `json:"items"`
	Sources []string                             `json:"sources"`
}

// GetSidebarLinkDefaults returns admin settings merged with all configured sources (memory only).
func GetSidebarLinkDefaults() SidebarLinkDefaultsSettings {
	doc := usersidebar.DocumentWithAllSources(EffectiveSidebarLinkDefaults())
	doc = usersidebar.FrontendDefaultsDocument(doc)
	return SidebarLinkDefaultsSettings{
		Items:   doc.Items,
		Sources: usersidebar.ConfiguredSourceNames(),
	}
}

// PatchSidebarLinkDefaults replaces the full sidebar link defaults document and resyncs users when needed.
func PatchSidebarLinkDefaults(doc usersidebar.SidebarLinkDefaultsDocument) error {
	if doc.Items == nil {
		doc.Items = []usersidebar.SidebarLinkDefaultItem{}
	}
	doc = usersidebar.NormalizeDefaultsDocument(doc)

	sidebarLinkDefaultsMu.Lock()
	prev := sidebarLinkDefaults
	if err := saveSidebarLinkDefaultsDocument(doc); err != nil {
		sidebarLinkDefaultsMu.Unlock()
		return fmt.Errorf("save sidebar link defaults: %w", err)
	}
	sidebarLinkDefaults = doc
	sidebarLinkDefaultsMu.Unlock()

	if sidebarLinkDefaultsEqual(prev, doc) {
		return nil
	}
	if err := ResyncSidebarLinkDefaultsForAllUsers(); err != nil {
		return err
	}
	return ResyncEnforcedSidebarLinksForAllUsers()
}

func sidebarLinkDefaultsEqual(a, b usersidebar.SidebarLinkDefaultsDocument) bool {
	return sameDefaultFlags(a, b, func(item usersidebar.SidebarLinkDefaultItem) bool { return item.Enabled }) &&
		sameDefaultFlags(a, b, func(item usersidebar.SidebarLinkDefaultItem) bool { return item.Enforced }) &&
		sameDefaultLinkTemplates(a, b)
}

func sameDefaultLinkTemplates(a, b usersidebar.SidebarLinkDefaultsDocument) bool {
	ma := defaultLinkTemplatesMap(a)
	mb := defaultLinkTemplatesMap(b)
	if len(ma) != len(mb) {
		return false
	}
	for k, va := range ma {
		vb, ok := mb[k]
		if !ok || !sidebarLinksEquivalent(va, vb) {
			return false
		}
	}
	return true
}

func defaultLinkTemplatesMap(doc usersidebar.SidebarLinkDefaultsDocument) map[string]users.SidebarLink {
	out := make(map[string]users.SidebarLink)
	for _, item := range doc.Items {
		prepared, ok := prepareSidebarDefaultLink(item.Link)
		if !ok {
			continue
		}
		out[usersidebar.LinkKey(prepared)] = prepared
	}
	return out
}

func sidebarLinksEquivalent(a, b users.SidebarLink) bool {
	return a.Name == b.Name && a.Category == b.Category && a.Target == b.Target && a.Icon == b.Icon && a.SourceName == b.SourceName
}

func sameDefaultFlags(before, after usersidebar.SidebarLinkDefaultsDocument, flag func(usersidebar.SidebarLinkDefaultItem) bool) bool {
	b := defaultFlagsMap(before, flag)
	a := defaultFlagsMap(after, flag)
	if len(b) != len(a) {
		return false
	}
	for k, v := range b {
		if a[k] != v {
			return false
		}
	}
	return true
}

func defaultFlagsMap(doc usersidebar.SidebarLinkDefaultsDocument, flag func(usersidebar.SidebarLinkDefaultItem) bool) map[string]bool {
	out := make(map[string]bool)
	for _, item := range doc.Items {
		prepared, ok := prepareSidebarDefaultLink(item.Link)
		if !ok {
			continue
		}
		out[usersidebar.LinkKey(prepared)] = flag(item)
	}
	return out
}

func prepareSidebarDefaultLink(link users.SidebarLink) (users.SidebarLink, bool) {
	normalized, _ := usersidebar.NormalizeSidebarLinks([]users.SidebarLink{link})
	if len(normalized) == 0 {
		return users.SidebarLink{}, false
	}
	return normalized[0], true
}
