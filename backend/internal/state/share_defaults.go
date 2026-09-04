package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const (
	shareDefaultsDefaultSettingKey  = "shareDefaults.default"
	shareDefaultsEnforcedDefaultKey = "shareDefaults.enforced.default"
)

var (
	shareDefaultsMu              sync.RWMutex
	shareDefaultsDefault         settings.ShareDefaults
	shareDefaultsEnforcedDefault settings.ShareDefaultsEnforcement
)

// InitShareDefaultsSettings loads persisted share defaults from SQLite and seeds from config when missing.
func InitShareDefaultsSettings() error {
	seed := settings.Config.ShareDefaults
	if _, err := sqlDb.GetSetting(shareDefaultsDefaultSettingKey); err != nil {
		if saveErr := sqlDb.SaveSetting(shareDefaultsDefaultSettingKey, seed); saveErr != nil {
			return fmt.Errorf("seed share defaults: %w", saveErr)
		}
	}
	if _, err := sqlDb.GetSetting(shareDefaultsEnforcedDefaultKey); err != nil {
		if saveErr := sqlDb.SaveSetting(shareDefaultsEnforcedDefaultKey, settings.ShareDefaultsEnforcement{}); saveErr != nil {
			return fmt.Errorf("seed enforced share defaults: %w", saveErr)
		}
	}

	defaults, err := loadShareDefaultsSetting(shareDefaultsDefaultSettingKey)
	if err != nil {
		return fmt.Errorf("load share defaults: %w", err)
	}
	enforcedDefault, err := loadShareDefaultsEnforcedSetting(shareDefaultsEnforcedDefaultKey)
	if err != nil {
		return fmt.Errorf("load enforced share defaults: %w", err)
	}

	shareDefaultsMu.Lock()
	shareDefaultsDefault = defaults
	shareDefaultsEnforcedDefault = enforcedDefault
	settings.Config.ShareDefaults = defaults
	shareDefaultsMu.Unlock()
	return nil
}

func loadShareDefaultsSetting(key string) (settings.ShareDefaults, error) {
	raw, err := sqlDb.GetSetting(key)
	if err != nil {
		return settings.ShareDefaults{}, err
	}
	var sd settings.ShareDefaults
	if err := json.Unmarshal(raw, &sd); err != nil {
		return settings.ShareDefaults{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return sd, nil
}

func loadShareDefaultsEnforcedSetting(key string) (settings.ShareDefaultsEnforcement, error) {
	raw, err := sqlDb.GetSetting(key)
	if err != nil {
		return settings.ShareDefaultsEnforcement{}, err
	}
	var enforced settings.ShareDefaultsEnforcement
	if err := json.Unmarshal(raw, &enforced); err != nil {
		return settings.ShareDefaultsEnforcement{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return enforced, nil
}

// GetShareDefaults returns the share defaults template.
func GetShareDefaults() settings.ShareDefaults {
	shareDefaultsMu.RLock()
	defer shareDefaultsMu.RUnlock()
	return shareDefaultsDefault
}

// GetEnforcedShareDefaults returns share enforcement flags.
func GetEnforcedShareDefaults() settings.ShareDefaultsEnforcement {
	shareDefaultsMu.RLock()
	defer shareDefaultsMu.RUnlock()
	return shareDefaultsEnforcedDefault
}

// PatchShareDefaults merges patch JSON into share defaults and persists.
func PatchShareDefaults(patchJSON []byte) error {
	shareDefaultsMu.Lock()
	merged, mergeErr := settings.MergeShareDefaultsPatchJSON(shareDefaultsDefault, patchJSON)
	if mergeErr != nil {
		shareDefaultsMu.Unlock()
		return mergeErr
	}
	if saveErr := sqlDb.SaveSetting(shareDefaultsDefaultSettingKey, merged); saveErr != nil {
		shareDefaultsMu.Unlock()
		return fmt.Errorf("save share defaults: %w", saveErr)
	}
	shareDefaultsDefault = merged
	settings.Config.ShareDefaults = merged
	shareDefaultsMu.Unlock()
	return nil
}

// PatchShareDefaultsEnforced merges enforcement patch JSON and persists.
func PatchShareDefaultsEnforced(patchJSON []byte) error {
	shareDefaultsMu.Lock()
	merged, mergeErr := settings.MergeShareEnforcedPatchJSON(shareDefaultsEnforcedDefault, patchJSON)
	if mergeErr != nil {
		shareDefaultsMu.Unlock()
		return mergeErr
	}
	if saveErr := sqlDb.SaveSetting(shareDefaultsEnforcedDefaultKey, merged); saveErr != nil {
		shareDefaultsMu.Unlock()
		return fmt.Errorf("save enforced share defaults: %w", saveErr)
	}
	shareDefaultsEnforcedDefault = merged
	shareDefaultsMu.Unlock()
	return nil
}
