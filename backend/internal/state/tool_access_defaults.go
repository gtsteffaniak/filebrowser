package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/internal/toolaccess"
)

const toolAccessDefaultsSettingKey = "toolAccessDefaults"

var (
	toolAccessDefaultsMu            sync.RWMutex
	toolAccessDefaultsPatchMu       sync.Mutex
	toolAccessDefaults              toolaccess.ToolAccessDefaultsDocument
	toolAccessDefaultsNeedResync    bool
	toolAccessDefaultsResyncPending bool
	// errInjectResyncToolAccessDefaults is set by tests in this package only.
	errInjectResyncToolAccessDefaults error
)

// InitToolAccessDefaults loads persisted tool access defaults into memory.
func InitToolAccessDefaults() error {
	if sqlDb == nil {
		return fmt.Errorf("sqlDb not initialized")
	}
	doc, found, err := loadToolAccessDefaultsDocument()
	if err != nil {
		return err
	}
	if !found {
		doc = toolaccess.InitialToolAccessDefaultsDocument()
		if saveErr := saveToolAccessDefaultsDocument(doc); saveErr != nil {
			return saveErr
		}
	} else {
		ensured, changed := toolaccess.EnsureCatalogInDefaults(doc)
		if changed {
			if saveErr := saveToolAccessDefaultsDocument(ensured); saveErr != nil {
				return saveErr
			}
			doc = ensured
			toolAccessDefaultsNeedResync = true
		}
	}
	toolAccessDefaultsMu.Lock()
	toolAccessDefaults = doc
	toolAccessDefaultsMu.Unlock()
	if !found {
		toolAccessDefaultsNeedResync = true
	}
	return nil
}

func loadToolAccessDefaultsDocument() (toolaccess.ToolAccessDefaultsDocument, bool, error) {
	raw, err := sqlDb.GetSetting(toolAccessDefaultsSettingKey)
	if err != nil {
		if err.Error() == fmt.Sprintf("setting not found: %s", toolAccessDefaultsSettingKey) {
			return toolaccess.ToolAccessDefaultsDocument{}, false, nil
		}
		return toolaccess.ToolAccessDefaultsDocument{}, false, err
	}
	var doc toolaccess.ToolAccessDefaultsDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return toolaccess.ToolAccessDefaultsDocument{}, false, fmt.Errorf("parse %s: %w", toolAccessDefaultsSettingKey, err)
	}
	if doc.Items == nil {
		doc.Items = []toolaccess.ToolAccessDefaultItem{}
	}
	return doc, true, nil
}

func saveToolAccessDefaultsDocument(doc toolaccess.ToolAccessDefaultsDocument) error {
	if doc.Items == nil {
		doc.Items = []toolaccess.ToolAccessDefaultItem{}
	}
	return sqlDb.SaveSetting(toolAccessDefaultsSettingKey, doc)
}

// EffectiveToolAccessDefaults returns the in-memory tool access defaults document.
func EffectiveToolAccessDefaults() toolaccess.ToolAccessDefaultsDocument {
	toolAccessDefaultsMu.RLock()
	defer toolAccessDefaultsMu.RUnlock()
	doc := toolAccessDefaults
	if doc.Items != nil {
		doc.Items = append([]toolaccess.ToolAccessDefaultItem(nil), doc.Items...)
	}
	return doc
}

// GetToolAccessDefaults returns the admin API response for GET /api/settings/tool-access-defaults.
func GetToolAccessDefaults() toolaccess.ToolAccessDefaultsDocument {
	doc := EffectiveToolAccessDefaults()
	ensured, _ := toolaccess.EnsureCatalogInDefaults(doc)
	return toolaccess.NormalizeDefaultsDocument(ensured)
}

// GetToolAccessDefaultsForUser returns tool access defaults visible to the user (same document for all users).
func GetToolAccessDefaultsForUser() toolaccess.ToolAccessDefaultsDocument {
	return GetToolAccessDefaults()
}

// PatchToolAccessDefaults replaces the full tool access defaults document and resyncs users when needed.
func PatchToolAccessDefaults(doc toolaccess.ToolAccessDefaultsDocument) error {
	toolAccessDefaultsPatchMu.Lock()
	defer toolAccessDefaultsPatchMu.Unlock()

	if doc.Items == nil {
		doc.Items = []toolaccess.ToolAccessDefaultItem{}
	}
	doc = toolaccess.NormalizeDefaultsDocument(doc)

	toolAccessDefaultsMu.Lock()
	prev := toolAccessDefaults
	docChanged := !toolAccessDefaultsEqual(prev, doc)
	if docChanged {
		if err := saveToolAccessDefaultsDocument(doc); err != nil {
			toolAccessDefaultsMu.Unlock()
			return fmt.Errorf("save tool access defaults: %w", err)
		}
		toolAccessDefaults = doc
	}
	syncDoc := toolAccessDefaults
	needsResync := docChanged || toolAccessDefaultsResyncPending
	toolAccessDefaultsMu.Unlock()

	if !needsResync {
		return nil
	}
	if err := resyncToolAccessForAllUsers(syncDoc); err != nil {
		toolAccessDefaultsResyncPending = true
		return err
	}
	toolAccessDefaultsResyncPending = false
	return nil
}

func toolAccessDefaultsEqual(a, b toolaccess.ToolAccessDefaultsDocument) bool {
	ma := toolAccessFlagsMap(a)
	mb := toolAccessFlagsMap(b)
	if len(ma) != len(mb) {
		return false
	}
	for k, va := range ma {
		if mb[k] != va {
			return false
		}
	}
	return true
}

func toolAccessFlagsMap(doc toolaccess.ToolAccessDefaultsDocument) map[string]string {
	out := make(map[string]string)
	for _, item := range doc.Items {
		out[string(item.ToolID)] = fmt.Sprintf("%t:%t", item.Enabled, item.Enforced)
	}
	return out
}
