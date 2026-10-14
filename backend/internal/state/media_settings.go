package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

const mediaSettingsDefaultKey = "media.default"

var (
	mediaSettingsMu      sync.RWMutex
	mediaSettingsDefault settings.MediaSettings
)

// InitMediaSettings loads persisted runtime media settings from SQLite and seeds from config when missing.
func InitMediaSettings() error {
	if _, err := sqlDb.GetSetting(mediaSettingsDefaultKey); err != nil {
		seed := settings.SanitizeMediaSettings(settings.CurrentMediaSettings())
		if saveErr := sqlDb.SaveSetting(mediaSettingsDefaultKey, seed); saveErr != nil {
			return fmt.Errorf("seed media settings: %w", saveErr)
		}
	}

	loaded, err := loadMediaSettingsSetting(mediaSettingsDefaultKey)
	if err != nil {
		return fmt.Errorf("load media settings: %w", err)
	}
	loaded = settings.SanitizeMediaSettings(loaded)
	if migrated, changed := settings.MigrateLegacyMediaDefaults(loaded); changed {
		loaded = migrated
		if saveErr := sqlDb.SaveSetting(mediaSettingsDefaultKey, loaded); saveErr != nil {
			logger.Warningf("failed to persist media settings migration: %v", saveErr)
		}
	}

	mediaSettingsMu.Lock()
	mediaSettingsDefault = loaded
	settings.ApplyMediaSettingsIfChanged(loaded)
	mediaSettingsMu.Unlock()
	return nil
}

func loadMediaSettingsSetting(key string) (settings.MediaSettings, error) {
	raw, err := sqlDb.GetSetting(key)
	if err != nil {
		return settings.MediaSettings{}, err
	}
	var ms settings.MediaSettings
	if err := json.Unmarshal(raw, &ms); err != nil {
		return settings.MediaSettings{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return ms, nil
}

// GetMediaSettings returns the runtime media settings.
func GetMediaSettings() settings.MediaSettings {
	mediaSettingsMu.RLock()
	defer mediaSettingsMu.RUnlock()
	return mediaSettingsDefault
}

// PatchMediaSettings merges patch JSON into the media settings, persists, and applies them.
func PatchMediaSettings(patchJSON []byte) error {
	mediaSettingsMu.Lock()
	merged, mergeErr := settings.MergeMediaSettingsPatchJSON(mediaSettingsDefault, patchJSON)
	if mergeErr != nil {
		mediaSettingsMu.Unlock()
		return mergeErr
	}
	merged = settings.SanitizeMediaSettings(merged)

	if saveErr := sqlDb.SaveSetting(mediaSettingsDefaultKey, merged); saveErr != nil {
		mediaSettingsMu.Unlock()
		return fmt.Errorf("save media settings: %w", saveErr)
	}

	mediaSettingsDefault = merged
	settings.ApplyMediaSettings(merged)
	mediaSettingsMu.Unlock()
	return nil
}
