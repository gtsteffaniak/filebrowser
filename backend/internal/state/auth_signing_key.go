package state

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

const authSigningKeySetting = "auth.signingKey"

// InitAuthSigningKey loads or creates the JWT HMAC secret and persists it in SQLite settings.
func InitAuthSigningKey() error {
	if sqlDb == nil {
		return fmt.Errorf("database not initialized")
	}

	explicitKey := settings.Config.Auth.Key

	storedKey, storedErr := loadPersistedAuthSigningKey()
	if storedErr == nil && storedKey != "" {
		if explicitKey == "" {
			settings.Config.Auth.Key = storedKey
		} else if explicitKey != storedKey {
			logger.Warning("auth signing key from config/env differs from value stored in the application database; using config/env for this process")
		}
	}

	if settings.Config.Auth.Key == "" {
		boltPath := settings.Config.Server.DatabaseV2.MigrateFrom
		if key, err := ReadAuthSigningKeyFromBoltFile(boltPath); err == nil && key != "" {
			settings.Config.Auth.Key = key
			logger.Info("Recovered auth signing key from legacy database file")
		}
	}

	if settings.Config.Auth.Key == "" {
		key := utils.GenerateKey()
		if key == "" {
			return fmt.Errorf("failed to generate auth signing key")
		}
		settings.Config.Auth.Key = key
		logger.Info("Generated a new auth signing key (stored in the application database)")
	}

	if storedErr != nil || storedKey == "" {
		if err := persistAuthSigningKey(settings.Config.Auth.Key); err != nil {
			return fmt.Errorf("persist auth signing key: %w", err)
		}
	}

	if len(settings.Config.Auth.Key) == 0 {
		return fmt.Errorf("auth signing key is not configured")
	}
	return nil
}

func loadPersistedAuthSigningKey() (string, error) {
	raw, err := sqlDb.GetSetting(authSigningKeySetting)
	if err != nil {
		return "", err
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return "", fmt.Errorf("parse auth signing key: %w", err)
	}
	if encoded == "" {
		return "", errors.New("empty auth signing key in database")
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Legacy row: raw secret string (may be invalid UTF-8).
		return encoded, nil
	}
	if len(decoded) == 0 {
		return "", errors.New("empty auth signing key in database")
	}
	return string(decoded), nil
}

func persistAuthSigningKey(key string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(key))
	return sqlDb.SaveSetting(authSigningKeySetting, encoded)
}
