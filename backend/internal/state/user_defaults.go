package state

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

const (
	userDefaultsDefaultSettingKey  = "userDefaults.default"
	userDefaultsEnforcedDefaultKey = "userDefaults.enforced.default"

	userDefaultsLoginKeyPrefix      = "userDefaults.login."
	userDefaultsEnforcedLoginPrefix = "userDefaults.enforced.login."
)

var (
	userDefaultsMu              sync.RWMutex
	userDefaultsDefault         settings.UserDefaults
	userDefaultsEnforcedDefault settings.UserDefaultsEnforcement
)

// InitUserDefaultsSettings loads persisted user defaults from SQLite and seeds from config when missing.
func InitUserDefaultsSettings() error {
	seed := settings.Config.UserDefaults
	_, existingDefaultsErr := sqlDb.GetSetting(userDefaultsDefaultSettingKey)
	if settings.Env.ConfigUserDefaultsSpecified && existingDefaultsErr == nil {
		logger.Warning("userDefaults in the config file is deprecated; manage defaults in Settings → Users → User defaults. Values in the database are authoritative after the initial seed.")
	}

	if existingDefaultsErr != nil {
		if err := sqlDb.SaveSetting(userDefaultsDefaultSettingKey, seed); err != nil {
			return fmt.Errorf("seed user defaults: %w", err)
		}
	}

	if _, err := sqlDb.GetSetting(userDefaultsEnforcedDefaultKey); err != nil {
		enforced := settings.UserDefaultsEnforcement{}
		if len(settings.Env.ConfigUserDefaultsSpecifiedPaths) > 0 {
			settings.ApplyEnforcementFromPaths(&enforced, settings.Env.ConfigUserDefaultsSpecifiedPaths)
		}
		if saveErr := sqlDb.SaveSetting(userDefaultsEnforcedDefaultKey, enforced); saveErr != nil {
			return fmt.Errorf("seed enforced user defaults: %w", saveErr)
		}
	}

	if err := migrateLoginScopedUserDefaultsToUniversal(); err != nil {
		return fmt.Errorf("migrate scoped user defaults: %w", err)
	}

	defaults, err := loadUserDefaultsSetting(userDefaultsDefaultSettingKey)
	if err != nil {
		return fmt.Errorf("load user defaults: %w", err)
	}

	enforcedDefault, err := loadUserDefaultsEnforcedSetting(userDefaultsEnforcedDefaultKey)
	if err != nil {
		return fmt.Errorf("load enforced user defaults: %w", err)
	}

	userDefaultsMu.Lock()
	userDefaultsDefault = defaults
	userDefaultsEnforcedDefault = enforcedDefault
	settings.Config.UserDefaults = defaults
	userDefaultsMu.Unlock()

	return nil
}

func migrateLoginScopedUserDefaultsToUniversal() error {
	all, err := sqlDb.GetAllSettings()
	if err != nil {
		return err
	}

	if err := mergeAndDeleteLoginScopedSettings(
		all,
		userDefaultsLoginKeyPrefix,
		userDefaultsDefaultSettingKey,
		func(base settings.UserDefaults, patch []byte) (settings.UserDefaults, error) {
			return settings.MergeUserDefaultsPatchJSON(base, patch)
		},
	); err != nil {
		return err
	}

	return mergeAndDeleteLoginScopedEnforced(all)
}

func mergeAndDeleteLoginScopedEnforced(all map[string][]byte) error {
	base, err := loadUserDefaultsEnforcedSetting(userDefaultsEnforcedDefaultKey)
	if err != nil {
		return err
	}
	merged := base
	var deleted int
	for key, raw := range all {
		if !strings.HasPrefix(key, userDefaultsEnforcedLoginPrefix) {
			continue
		}
		scope := strings.TrimPrefix(key, userDefaultsEnforcedLoginPrefix)
		if scope == "" {
			continue
		}
		next, mergeErr := settings.MergeEnforcedPatchJSON(merged, raw)
		if mergeErr != nil {
			return fmt.Errorf("merge enforced login scope %s: %w", scope, mergeErr)
		}
		merged = next
		if err := sqlDb.DeleteSetting(key); err != nil {
			return fmt.Errorf("delete legacy enforced scope %s: %w", key, err)
		}
		deleted++
	}
	if deleted == 0 {
		return nil
	}
	if err := sqlDb.SaveSetting(userDefaultsEnforcedDefaultKey, merged); err != nil {
		return fmt.Errorf("save merged enforced user defaults: %w", err)
	}
	logger.Infof("migrated %d login-scoped enforced user default(s) into universal config", deleted)
	return nil
}

func mergeAndDeleteLoginScopedSettings[T any](
	all map[string][]byte,
	prefix string,
	targetKey string,
	merge func(base T, patch []byte) (T, error),
) error {
	rawBase, err := sqlDb.GetSetting(targetKey)
	if err != nil {
		return err
	}
	var merged T
	if err := json.Unmarshal(rawBase, &merged); err != nil {
		return fmt.Errorf("parse %s: %w", targetKey, err)
	}
	var deleted int
	for key, raw := range all {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		scope := strings.TrimPrefix(key, prefix)
		if scope == "" {
			continue
		}
		next, mergeErr := merge(merged, raw)
		if mergeErr != nil {
			return fmt.Errorf("merge login scope %s: %w", scope, mergeErr)
		}
		merged = next
		if err := sqlDb.DeleteSetting(key); err != nil {
			return fmt.Errorf("delete legacy scope %s: %w", key, err)
		}
		deleted++
	}
	if deleted == 0 {
		return nil
	}
	if err := sqlDb.SaveSetting(targetKey, merged); err != nil {
		return fmt.Errorf("save merged user defaults: %w", err)
	}
	logger.Infof("migrated %d login-scoped user default(s) into universal config", deleted)
	return nil
}

func loadUserDefaultsSetting(key string) (settings.UserDefaults, error) {
	raw, err := sqlDb.GetSetting(key)
	if err != nil {
		return settings.UserDefaults{}, err
	}
	var ud settings.UserDefaults
	if err := json.Unmarshal(raw, &ud); err != nil {
		return settings.UserDefaults{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return ud, nil
}

func loadUserDefaultsEnforcedSetting(key string) (settings.UserDefaultsEnforcement, error) {
	raw, err := sqlDb.GetSetting(key)
	if err != nil {
		return settings.UserDefaultsEnforcement{}, err
	}
	var enforced settings.UserDefaultsEnforcement
	if err := json.Unmarshal(raw, &enforced); err != nil {
		return settings.UserDefaultsEnforcement{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return enforced, nil
}

// GetDefaultUserDefaults returns the universal user defaults template.
func GetDefaultUserDefaults() settings.UserDefaults {
	return EffectiveUserDefaults()
}

// GetUserDefaults returns the universal user defaults template.
func GetUserDefaults() settings.UserDefaults {
	return EffectiveUserDefaults()
}

// GetEnforcedUserDefaults returns universal enforcement flags.
func GetEnforcedUserDefaults() settings.UserDefaultsEnforcement {
	return EffectiveEnforced()
}

// PatchUserDefaults merges patch JSON into the universal defaults and persists.
func PatchUserDefaults(patchJSON []byte) error {
	if err := settings.ValidateUserDefaultsPatchNotConfigLocked(patchJSON); err != nil {
		return err
	}
	userDefaultsMu.Lock()
	merged, mergeErr := settings.MergeUserDefaultsPatchJSON(userDefaultsDefault, patchJSON)
	if mergeErr != nil {
		userDefaultsMu.Unlock()
		return mergeErr
	}
	if saveErr := sqlDb.SaveSetting(userDefaultsDefaultSettingKey, merged); saveErr != nil {
		userDefaultsMu.Unlock()
		return fmt.Errorf("save user defaults: %w", saveErr)
	}
	userDefaultsDefault = merged
	settings.Config.UserDefaults = merged
	userDefaultsMu.Unlock()

	return nil
}

// PatchUserDefaultsEnforced merges enforcement patch JSON into the universal config.
func PatchUserDefaultsEnforced(patchJSON []byte) error {
	if err := settings.ValidateUserDefaultsPatchNotConfigLocked(patchJSON); err != nil {
		return err
	}
	userDefaultsMu.Lock()
	merged, mergeErr := settings.MergeEnforcedPatchJSON(userDefaultsEnforcedDefault, patchJSON)
	if mergeErr != nil {
		userDefaultsMu.Unlock()
		return mergeErr
	}
	if saveErr := sqlDb.SaveSetting(userDefaultsEnforcedDefaultKey, merged); saveErr != nil {
		userDefaultsMu.Unlock()
		return fmt.Errorf("save enforced user defaults: %w", saveErr)
	}
	userDefaultsEnforcedDefault = merged
	userDefaultsMu.Unlock()

	return ResyncEnforcedDefaultsForAllUsers()
}

// EffectiveEnforced returns universal enforcement flags.
func EffectiveEnforced() settings.UserDefaultsEnforcement {
	userDefaultsMu.RLock()
	defer userDefaultsMu.RUnlock()
	return userDefaultsEnforcedDefault
}

// EffectiveUserDefaults returns the universal defaults template for all users.
func EffectiveUserDefaults() settings.UserDefaults {
	userDefaultsMu.RLock()
	defer userDefaultsMu.RUnlock()
	return userDefaultsDefault
}

// ApplyUserDefaults applies persisted defaults to a user.
func ApplyUserDefaults(u *users.User) {
	settings.ApplyUserDefaultsFrom(u, EffectiveUserDefaults())
}
