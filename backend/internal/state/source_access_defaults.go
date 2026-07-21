package state

import (
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const sourceAccessDefaultsSettingKey = "sourceAccessDefaults"

// InitSourceAccessDefaults loads persisted source access defaults and applies them to every source.
func InitSourceAccessDefaults() error {
	if sqlDb == nil {
		return fmt.Errorf("sqlDb not initialized")
	}

	perms, found, err := loadSourceAccessDefaultsSetting()
	if err != nil {
		return err
	}
	if !found {
		perms = deriveInitialSourceAccessDefaults()
		if saveErr := sqlDb.SaveSetting(sourceAccessDefaultsSettingKey, perms); saveErr != nil {
			return saveErr
		}
	}

	settings.ApplySourceAccessDefaultsToAllSources(perms)
	if !found {
		return stripLegacyFilePermissionsFromUserDefaults()
	}
	return nil
}

func deriveInitialSourceAccessDefaults() users.SourceFilePermissions {
	for _, src := range settings.Config.Server.Sources {
		if src != nil && !src.Config.DefaultPermissions.IsUnset() {
			return settings.NormalizeSourceFilePermissions(src.Config.DefaultPermissions)
		}
	}
	return settings.BuiltinDefaultSourceFilePermissions()
}

func loadSourceAccessDefaultsSetting() (users.SourceFilePermissions, bool, error) {
	raw, err := sqlDb.GetSetting(sourceAccessDefaultsSettingKey)
	if err != nil {
		if err.Error() == fmt.Sprintf("setting not found: %s", sourceAccessDefaultsSettingKey) {
			return users.SourceFilePermissions{}, false, nil
		}
		return users.SourceFilePermissions{}, false, err
	}
	var perms users.SourceFilePermissions
	if err := json.Unmarshal(raw, &perms); err != nil {
		return users.SourceFilePermissions{}, false, fmt.Errorf("parse %s: %w", sourceAccessDefaultsSettingKey, err)
	}
	return settings.NormalizeSourceFilePermissions(perms), true, nil
}

// GetSourceAccessDefaults returns the global default file permissions for sources.
func GetSourceAccessDefaults() users.SourceFilePermissions {
	return settings.DefaultSourceFilePermissions()
}

// SourceSettings is the admin API payload for GET/PATCH /api/settings/source.
type SourceSettings struct {
	DefaultPermissions users.SourceFilePermissions `json:"defaultPermissions"`
}

// GetSourceSettings returns admin-editable source-wide settings.
func GetSourceSettings() SourceSettings {
	return SourceSettings{
		DefaultPermissions: GetSourceAccessDefaults(),
	}
}

// SetSourceAccessDefaults persists and applies new global source file permission defaults.
func SetSourceAccessDefaults(perms users.SourceFilePermissions) error {
	if sqlDb == nil {
		return fmt.Errorf("sqlDb not initialized")
	}
	normalized := settings.NormalizeSourceFilePermissions(users.MarkSourceFilePermissionsConfigured(perms))
	if err := sqlDb.SaveSetting(sourceAccessDefaultsSettingKey, normalized); err != nil {
		return err
	}
	settings.ApplySourceAccessDefaultsToAllSources(normalized)
	return nil
}

func stripLegacyFilePermissionsFromUserDefaults() error {
	patch := map[string]interface{}{
		"account": map[string]interface{}{
			"permissions": map[string]interface{}{
				"modify":   nil,
				"create":   nil,
				"delete":   nil,
				"download": nil,
				"view":     nil,
			},
		},
	}
	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	merged, mergeErr := settings.MergeUserDefaultsPatchJSON(userDefaultsDefault, patchJSON)
	if mergeErr != nil {
		return mergeErr
	}
	userDefaultsDefault = merged
	settings.Config.UserDefaults = merged
	if saveErr := sqlDb.SaveSetting(userDefaultsDefaultSettingKey, merged); saveErr != nil {
		return saveErr
	}
	enfPatch := map[string]interface{}{
		"account": map[string]interface{}{
			"permissions": map[string]interface{}{
				"modify":   false,
				"create":   false,
				"delete":   false,
				"download": false,
				"view":     false,
			},
		},
	}
	enfJSON, err := json.Marshal(enfPatch)
	if err != nil {
		return err
	}
	mergedEnf, mergeErr := settings.MergeEnforcedPatchJSON(userDefaultsEnforcedDefault, enfJSON)
	if mergeErr != nil {
		return mergeErr
	}
	userDefaultsEnforcedDefault = mergedEnf
	if saveErr := sqlDb.SaveSetting(userDefaultsEnforcedDefaultKey, mergedEnf); saveErr != nil {
		return saveErr
	}
	return nil
}
