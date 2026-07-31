package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// MigrateFromDefault is the config value that resolves server.database.migrateFrom from
// the FILEBROWSER_DATABASE environment variable (legacy BoltDB path).
const MigrateFromDefault = "default"

// ResolveDatabasePaths normalizes server.database.path and server.database.migrateFrom after
// config load. Relative paths are resolved against the config file directory (not CWD).
func ResolveDatabasePaths(configFile string) error {
	configDir, err := configFileDir(configFile)
	if err != nil {
		return err
	}
	Env.ConfigDir = configDir

	db := &Config.Server.DatabaseV2
	if strings.TrimSpace(db.Path) == "" {
		if v := os.Getenv("FILEBROWSER_DATABASE_PATH"); v != "" {
			db.Path = v
		} else {
			db.Path = "filebrowser.sqlite"
		}
	}

	migrateFrom := strings.TrimSpace(db.MigrateFrom)
	legacyEnv := strings.TrimSpace(os.Getenv("FILEBROWSER_DATABASE"))

	if migrateFrom == MigrateFromDefault {
		if legacyEnv == "" {
			return fmt.Errorf(
				`server.database.migrateFrom is %q but FILEBROWSER_DATABASE is not set. `+
					`Set FILEBROWSER_DATABASE to your legacy BoltDB path (Docker images often use `+
					`/home/filebrowser/data/database.db) or use an explicit migrateFrom path`,
				MigrateFromDefault,
			)
		}
		db.MigrateFrom = legacyEnv
	} else if legacyEnv != "" {
		logger.Fatalf(
			"FILEBROWSER_DATABASE environment variable is deprecated. " +
				`Remove it, or set server.database.migrateFrom to "default" to migrate from that path on first start.`,
		)
	}

	db.Path = resolveDatabasePath(db.Path, configDir)
	if db.MigrateFrom != "" {
		db.MigrateFrom = resolveDatabasePath(db.MigrateFrom, configDir)
	}
	return nil
}

func configFileDir(configFile string) (string, error) {
	if configFile == "" {
		wd, err := os.Getwd()
		if err != nil {
			return ".", fmt.Errorf("resolve config directory: %w", err)
		}
		return wd, nil
	}
	expanded := configFile
	if strings.HasPrefix(configFile, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		expanded = filepath.Join(home, configFile[2:])
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("resolve config file path: %w", err)
	}
	return filepath.Dir(abs), nil
}

func resolveDatabasePath(path, configDir string) string {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(configDir, path)
}

// LegacyDatabasePathInConfigDir returns the path to an unrenamed legacy database.db next to the config file.
func LegacyDatabasePathInConfigDir() string {
	if Env.ConfigDir != "" {
		return filepath.Join(Env.ConfigDir, "database.db")
	}
	return "database.db"
}
