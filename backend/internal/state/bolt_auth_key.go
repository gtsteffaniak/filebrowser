package state

import (
	"os"

	storm "github.com/asdine/storm/v3"
)

// boltSettingsAuth is the legacy Bolt "config"/"settings" document shape we need for migration.
type boltSettingsAuth struct {
	Auth struct {
		Key string `json:"key"`
	} `json:"auth"`
}

// AuthSigningKeyFromBoltDB reads auth.key from an open legacy Bolt database.
func AuthSigningKeyFromBoltDB(db *storm.DB) (string, error) {
	if db == nil {
		return "", os.ErrNotExist
	}
	var set boltSettingsAuth
	if err := db.Get("config", "settings", &set); err != nil {
		if err == storm.ErrNotFound {
			return "", os.ErrNotExist
		}
		return "", err
	}
	if set.Auth.Key == "" {
		return "", os.ErrNotExist
	}
	return set.Auth.Key, nil
}

// ReadAuthSigningKeyFromBoltFile reads the legacy BoltDB settings auth key (config/settings bucket).
func ReadAuthSigningKeyFromBoltFile(path string) (string, error) {
	if path == "" {
		return "", os.ErrNotExist
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() == 0 {
		return "", os.ErrNotExist
	}

	db, err := storm.Open(path)
	if err != nil {
		return "", err
	}
	defer db.Close()

	return AuthSigningKeyFromBoltDB(db)
}
