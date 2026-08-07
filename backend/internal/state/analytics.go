package state

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

const (
	installationIDSettingKey = "analytics.installation_id"
	analyticsEnabledKey      = "analytics.enabled"
)

var (
	analyticsEnabledMu sync.RWMutex
	analyticsEnabled   bool
)

// InitAnalyticsSettings loads persisted analytics preferences from SQLite.
// Default is disabled for new installations.
func InitAnalyticsSettings() error {
	enabled, err := loadAnalyticsEnabledFromDB()
	if err != nil {
		return err
	}
	analyticsEnabledMu.Lock()
	analyticsEnabled = enabled
	analyticsEnabledMu.Unlock()
	return nil
}

// IsAnalyticsEnabled reports whether anonymous deployment analytics is enabled.
func IsAnalyticsEnabled() bool {
	analyticsEnabledMu.RLock()
	defer analyticsEnabledMu.RUnlock()
	return analyticsEnabled
}

// SetAnalyticsEnabled persists the admin analytics toggle.
func SetAnalyticsEnabled(enabled bool) error {
	if err := sqlDb.SaveSetting(analyticsEnabledKey, enabled); err != nil {
		return fmt.Errorf("save analytics enabled: %w", err)
	}
	analyticsEnabledMu.Lock()
	analyticsEnabled = enabled
	analyticsEnabledMu.Unlock()
	return nil
}

func loadAnalyticsEnabledFromDB() (bool, error) {
	raw, err := sqlDb.GetSetting(analyticsEnabledKey)
	if err != nil {
		return false, nil
	}
	var enabled bool
	if unmarshalErr := json.Unmarshal(raw, &enabled); unmarshalErr != nil {
		return false, fmt.Errorf("parse analytics enabled: %w", unmarshalErr)
	}
	return enabled, nil
}

// GetOrCreateInstallationID returns a stable UUID v4 for this deployment.
func GetOrCreateInstallationID() (string, error) {
	raw, err := sqlDb.GetSetting(installationIDSettingKey)
	if err == nil {
		var id string
		if unmarshalErr := json.Unmarshal(raw, &id); unmarshalErr == nil && id != "" {
			return id, nil
		}
	}

	id := uuid.New().String()
	if err := sqlDb.SaveSetting(installationIDSettingKey, id); err != nil {
		return "", fmt.Errorf("save installation id: %w", err)
	}
	return id, nil
}

// CountAccessRules returns the total number of persisted access rules.
func CountAccessRules() (int, error) {
	allRules, err := sqlDb.GetAllAccessRules()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, byPath := range allRules {
		total += len(byPath)
	}
	return total, nil
}
