package state

import (
	"encoding/json"
	"fmt"
	"strings"
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

	// errInjectShareDefaultsEnforcedSave is test-only; simulates enforced save failure inside a transaction.
	errInjectShareDefaultsEnforcedSave error
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
	return patchShareDefaultsLocked(patchJSON, nil)
}

// PatchShareDefaultsEnforced merges enforcement patch JSON and persists.
func PatchShareDefaultsEnforced(patchJSON []byte) error {
	return patchShareDefaultsLocked(nil, patchJSON)
}

// PatchShareDefaultsCombined merges values and enforcement patches atomically.
func PatchShareDefaultsCombined(valuesPatch, enforcedPatch []byte) error {
	return patchShareDefaultsLocked(valuesPatch, enforcedPatch)
}

// IsShareDefaultsPersistenceError reports whether a patch error came from storage.
func IsShareDefaultsPersistenceError(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "save ")
}

func patchShareDefaultsLocked(valuesPatch, enforcedPatch []byte) error {
	shareDefaultsMu.Lock()

	newValues := shareDefaultsDefault
	newEnforced := shareDefaultsEnforcedDefault

	if len(valuesPatch) > 0 {
		merged, mergeErr := settings.MergeShareDefaultsPatchJSON(shareDefaultsDefault, valuesPatch)
		if mergeErr != nil {
			shareDefaultsMu.Unlock()
			return mergeErr
		}
		newValues = merged
	}
	if len(enforcedPatch) > 0 {
		merged, mergeErr := settings.MergeShareEnforcedPatchJSON(shareDefaultsEnforcedDefault, enforcedPatch)
		if mergeErr != nil {
			shareDefaultsMu.Unlock()
			return mergeErr
		}
		newEnforced = merged
	}

	saveValues := len(valuesPatch) > 0
	saveEnforced := len(enforcedPatch) > 0
	if saveValues && saveEnforced {
		if saveErr := saveShareDefaultsCombined(newValues, newEnforced); saveErr != nil {
			shareDefaultsMu.Unlock()
			return saveErr
		}
	} else if saveValues {
		if saveErr := sqlDb.SaveSetting(shareDefaultsDefaultSettingKey, newValues); saveErr != nil {
			shareDefaultsMu.Unlock()
			return fmt.Errorf("save share defaults: %w", saveErr)
		}
	} else if saveEnforced {
		if saveErr := sqlDb.SaveSetting(shareDefaultsEnforcedDefaultKey, newEnforced); saveErr != nil {
			shareDefaultsMu.Unlock()
			return fmt.Errorf("save enforced share defaults: %w", saveErr)
		}
	}

	shareDefaultsDefault = newValues
	shareDefaultsEnforcedDefault = newEnforced
	if len(valuesPatch) > 0 {
		settings.Config.ShareDefaults = newValues
	}
	shareDefaultsMu.Unlock()
	return nil
}

func saveShareDefaultsCombined(newValues settings.ShareDefaults, newEnforced settings.ShareDefaultsEnforcement) error {
	tx, err := sqlDb.BeginTx()
	if err != nil {
		return fmt.Errorf("save share defaults: %w", err)
	}
	if err := sqlDb.SaveSettingTx(tx, shareDefaultsDefaultSettingKey, newValues); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save share defaults: %w", err)
	}
	if errInjectShareDefaultsEnforcedSave != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save enforced share defaults: %w", errInjectShareDefaultsEnforcedSave)
	}
	if err := sqlDb.SaveSettingTx(tx, shareDefaultsEnforcedDefaultKey, newEnforced); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save enforced share defaults: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("save share defaults: %w", err)
	}
	return nil
}
