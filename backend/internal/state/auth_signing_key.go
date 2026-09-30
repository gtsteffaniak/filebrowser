package state

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

const authSigningKeySetting = "auth.signingKey"

// errAuthSigningKeyAbsent marks a missing persisted signing key. It is distinct
// from a corrupt row, which must abort startup instead of being overwritten.
var errAuthSigningKeyAbsent = errors.New("auth signing key not persisted")

// InitAuthSigningKey loads or creates the JWT HMAC secret and persists it in SQLite settings.
func InitAuthSigningKey() error {
	if sqlDb == nil {
		return fmt.Errorf("database not initialized")
	}

	key, err := resolveAuthSigningKey(settings.Config.Auth.Key)
	if err != nil {
		return err
	}
	settings.Config.Auth.Key = key
	return nil
}

// resolveAuthSigningKey returns the key to use without mutating config until the
// key is known to be valid. Only a missing persisted row counts as absence; a
// malformed row, an unreadable legacy database, or a configured key that differs
// from the persisted one are hard errors so nodes in the same deployment can
// never mint and verify with divergent keys.
func resolveAuthSigningKey(explicitKey string) (string, error) {
	storedKey, storedErr := loadPersistedAuthSigningKey()
	switch {
	case storedErr == nil && storedKey != "":
		if explicitKey == "" || explicitKey == storedKey {
			return storedKey, nil
		}
		return "", fmt.Errorf("auth signing key from config/env differs from the persisted database key; remove the configured key to use the stored key")
	case errors.Is(storedErr, errAuthSigningKeyAbsent):
		// No persisted key yet: persist the configured key or fall back below.
	default:
		return "", fmt.Errorf("load persisted auth signing key: %w", storedErr)
	}

	if explicitKey != "" {
		if err := persistAuthSigningKey(explicitKey); err != nil {
			return "", fmt.Errorf("persist auth signing key: %w", err)
		}
		return explicitKey, nil
	}

	boltPath := settings.Config.Server.DatabaseV2.MigrateFrom
	legacyKey, legacyErr := ReadAuthSigningKeyFromBoltFile(boltPath)
	switch {
	case legacyErr == nil && legacyKey != "":
		if err := persistAuthSigningKey(legacyKey); err != nil {
			return "", fmt.Errorf("persist auth signing key: %w", err)
		}
		logger.Info("Recovered auth signing key from legacy database file")
		return legacyKey, nil
	case errors.Is(legacyErr, os.ErrNotExist):
		// No legacy key to recover.
	default:
		return "", fmt.Errorf("read legacy auth signing key: %w", legacyErr)
	}

	generated := utils.GenerateKey()
	if generated == "" {
		return "", fmt.Errorf("failed to generate auth signing key")
	}
	if err := persistAuthSigningKey(generated); err != nil {
		return "", fmt.Errorf("persist auth signing key: %w", err)
	}
	logger.Info("Generated a new auth signing key (stored in the application database)")
	return generated, nil
}

func loadPersistedAuthSigningKey() (string, error) {
	raw, err := sqlDb.GetSetting(authSigningKeySetting)
	if err != nil {
		if errors.Is(err, sqldb.ErrSettingNotFound) {
			return "", errAuthSigningKeyAbsent
		}
		return "", err
	}
	var encoded string
	if err = json.Unmarshal(raw, &encoded); err != nil {
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
