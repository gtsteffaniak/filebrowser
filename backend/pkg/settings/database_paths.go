package settings

import (
	"os"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// MigrateFromDefault resolves server.database.migrateFrom from FILEBROWSER_DATABASE or database.db.
const MigrateFromDefault = "default"

const defaultLegacyDatabasePath = "database.db"

// ResolveDatabasePaths applies migrateFrom "default" and fills an empty SQLite path from existing defaults.
func ResolveDatabasePaths() error {
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
		if legacyEnv != "" {
			db.MigrateFrom = legacyEnv
		} else {
			db.MigrateFrom = defaultLegacyDatabasePath
		}
	} else if legacyEnv != "" {
		logger.Fatalf(
			"FILEBROWSER_DATABASE environment variable is deprecated. " +
				`Remove it, or set server.database.migrateFrom to "default" to migrate from that path on first start.`,
		)
	}
	return nil
}
