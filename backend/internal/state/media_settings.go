package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const (
	mediaSettingsDefaultKey = "media.default"
)

var (
	mediaSettingsMu      sync.RWMutex
	mediaSettingsDefault settings.FullAdminMedia
)

// saveMediaSettings is the only write path for media settings. It clears derived
// fields (EnabledProfiles, ImagePreview) so read-time values are never persisted.
func saveMediaSettings(key string, ms settings.FullAdminMedia) error {
	settings.ClearMediaDerived(&ms)
	return sqlDb.SaveSetting(key, ms)
}

// InitMediaSettings loads persisted runtime media settings from SQLite and seeds from config when missing.
func InitMediaSettings() error {
	if _, err := sqlDb.GetSetting(mediaSettingsDefaultKey); err != nil {
		seed := settings.SanitizeMediaSettings(settings.CurrentMediaSettings())
		if saveErr := saveMediaSettings(mediaSettingsDefaultKey, seed); saveErr != nil {
			return fmt.Errorf("seed media settings: %w", saveErr)
		}
	}

	loaded, err := loadMediaSettingsSetting(mediaSettingsDefaultKey)
	if err != nil {
		return fmt.Errorf("load media settings: %w", err)
	}
	loaded = settings.SanitizeMediaSettings(loaded)

	// Media settings pinned in the config file win over stored values, and are
	// locked in the admin UI.
	if settings.MediaSettingsLockedFromConfig() {
		overlaid, overlayErr := settings.ApplyConfigSpecifiedPathsToMedia(loaded, settings.ConfigMediaIntegration())
		if overlayErr != nil {
			return fmt.Errorf("overlay config media settings: %w", overlayErr)
		}
		// Re-sanitize: config-pinned values are overlaid after the load-time
		// sanitize, so clamps (e.g. the readRate floor) must run again.
		overlaid = settings.SanitizeMediaSettings(overlaid)
		if !settings.MediaSettingsEqual(loaded, overlaid) {
			if saveErr := saveMediaSettings(mediaSettingsDefaultKey, overlaid); saveErr != nil {
				return fmt.Errorf("save config-synced media settings: %w", saveErr)
			}
			loaded = overlaid
		}
	}

	mediaSettingsMu.Lock()
	mediaSettingsDefault = loaded
	settings.ApplyMediaSettingsIfChanged(loaded)
	mediaSettingsMu.Unlock()
	return nil
}

func loadMediaSettingsSetting(key string) (settings.FullAdminMedia, error) {
	raw, err := sqlDb.GetSetting(key)
	if err != nil {
		return settings.FullAdminMedia{}, err
	}
	var ms settings.FullAdminMedia
	if err := json.Unmarshal(raw, &ms); err != nil {
		return settings.FullAdminMedia{}, fmt.Errorf("parse %s: %w", key, err)
	}
	return ms, nil
}

// GetMediaSettings returns the stored runtime media settings. The value is deep-copied
// so callers cannot mutate the stored settings (or the live config they are
// applied to) through the conversion maps.
func GetMediaSettings() settings.FullAdminMedia {
	mediaSettingsMu.RLock()
	defer mediaSettingsMu.RUnlock()
	return settings.CloneMediaSettings(mediaSettingsDefault)
}

// PatchMediaSettings merges patch JSON into the media settings, persists, and applies them.
func PatchMediaSettings(patchJSON []byte) error {
	if lockErr := settings.ValidateMediaPatchNotConfigLocked(patchJSON); lockErr != nil {
		return lockErr
	}
	mediaSettingsMu.Lock()
	merged, mergeErr := settings.MergeMediaSettingsPatchJSON(mediaSettingsDefault, patchJSON)
	if mergeErr != nil {
		mediaSettingsMu.Unlock()
		return mergeErr
	}
	merged = settings.SanitizeMediaSettings(merged)
	// A patch may echo derived fields back (e.g. imagePreview from the admin UI).
	// Clear them before the value reaches memory or the database.
	settings.ClearMediaDerived(&merged)

	if saveErr := saveMediaSettings(mediaSettingsDefaultKey, merged); saveErr != nil {
		mediaSettingsMu.Unlock()
		return fmt.Errorf("save media settings: %w", saveErr)
	}

	mediaSettingsDefault = merged
	mediaSettingsMu.Unlock()

	// Applies to the in-memory config and, when an ffmpeg init input changed,
	// re-runs the hardware codec probe. Done outside the lock so media reads are
	// not blocked behind a multi-second probe.
	settings.ApplyMediaSettings(merged)
	return nil
}
