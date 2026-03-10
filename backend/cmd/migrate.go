package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// checkMigrationNeeded checks if migration from BoltDB to SQLite is needed
func checkMigrationNeeded() bool {
	oldDBPath := settings.Config.Server.DatabaseV2.MigrateFrom
	newDBPath := settings.Config.Server.DatabaseV2.Path

	// Check if old DB exists
	oldExists := false
	if oldDBPath != "" {
		if stat, err := os.Stat(oldDBPath); err == nil && stat.Size() > 0 {
			oldExists = true
		}
	}

	// Check if new DB exists
	newExists := false
	if stat, err := os.Stat(newDBPath); err == nil && stat.Size() > 0 {
		newExists = true
	}

	logger.Infof("Old database path: %s", oldDBPath)
	logger.Infof("New database path: %s", newDBPath)
	logger.Infof("Old database exists: %t", oldExists)
	logger.Infof("New database exists: %t", newExists)

	// Migration needed if old exists and new doesn't
	return oldExists && !newExists
}

// migrateFromBoltToSQLite migrates data from BoltDB to SQLite
func migrateFromBoltToSQLite() error {
	oldDBPath := settings.Config.Server.DatabaseV2.MigrateFrom
	newDBPath := settings.Config.Server.DatabaseV2.Path

	logger.Info("========================================")
	logger.Info("Starting migration from BoltDB to SQLite")
	logger.Info("========================================")

	// Create backup of old database
	backupPath := fmt.Sprintf("%s.pre-sqlite-migration.bak", oldDBPath)
	logger.Infof("Creating backup of old database at: %s", backupPath)
	err := fileutils.CopyFile(oldDBPath, backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	logger.Info("✓ Backup created successfully")

	// Open old BoltDB (read-only)
	logger.Infof("Opening old BoltDB from: %s", oldDBPath)
	oldDB, err := storm.Open(oldDBPath)
	if err != nil {
		return fmt.Errorf("failed to open old database: %w", err)
	}
	logger.Info("✓ Old database opened")

	// Initialize new SQLite database
	logger.Infof("Initializing new SQLite database at: %s", newDBPath)
	sqlStore, _, err := sqldb.NewSQLStore(newDBPath)
	if err != nil {
		oldDB.Close()
		return fmt.Errorf("failed to initialize new database: %w", err)
	}
	logger.Info("✓ New database initialized")

	// Ensure cleanup on error - remove incomplete database for retry
	migrationSuccess := false
	defer func() {
		oldDB.Close()
		sqlStore.Close()

		if !migrationSuccess {
			// Migration failed - remove the new database so it can be retried
			logger.Warning("Migration failed, cleaning up new database file for retry...")
			if err := os.Remove(newDBPath); err != nil && !os.IsNotExist(err) {
				logger.Errorf("Failed to remove incomplete database file: %v", err)
			} else {
				logger.Info("✓ Incomplete database file removed, migration can be retried on next startup")
			}
		}
	}()

	// Migrate users
	logger.Info("Migrating users...")
	if err := migrateUsers(oldDB, sqlStore); err != nil {
		return fmt.Errorf("failed to migrate users: %w", err)
	}

	// Migrate shares
	logger.Info("Migrating shares...")
	if err := migrateShares(oldDB, sqlStore); err != nil {
		return fmt.Errorf("failed to migrate shares: %w", err)
	}

	// Migrate access rules and groups
	logger.Info("Migrating access rules and groups...")
	if err := migrateAccessRules(oldDB, sqlStore); err != nil {
		return fmt.Errorf("failed to migrate access rules: %w", err)
	}

	// Migrate settings
	logger.Info("Migrating settings...")
	if err := migrateSettings(oldDB, sqlStore); err != nil {
		return fmt.Errorf("failed to migrate settings: %w", err)
	}

	// Migrate auth methods
	logger.Info("Migrating auth methods...")
	if err := migrateAuthMethods(oldDB, sqlStore); err != nil {
		return fmt.Errorf("failed to migrate auth methods: %w", err)
	}

	// Migrate index info
	logger.Info("Migrating index info...")
	if err := migrateIndexInfo(oldDB, sqlStore); err != nil {
		return fmt.Errorf("failed to migrate index info: %w", err)
	}

	// Mark migration as successful
	migrationSuccess = true

	logger.Info("========================================")
	logger.Info("Migration completed successfully!")
	logger.Info("========================================")
	logger.Infof("Old database backed up to: %s", backupPath)
	logger.Infof("New SQLite database created at: %s", newDBPath)

	return nil
}

// migrateUsers migrates all users from BoltDB to SQLite
func migrateUsers(oldDB *storm.DB, sqlStore *sqldb.SQLStore) error {
	var usersList []*users.User
	err := oldDB.All(&usersList)
	if err != nil {
		if err.Error() == "not found" {
			logger.Info("  No users to migrate")
			return nil
		}
		return fmt.Errorf("failed to read users from old DB: %w", err)
	}

	for _, user := range usersList {
		// Reset ID to 0 so SQLite assigns new auto-increment IDs
		oldID := user.ID
		user.ID = 0
		
		err := sqlStore.SaveUser(user)
		if err != nil {
			return fmt.Errorf("failed to save user %s (old ID: %d): %w", user.Username, oldID, err)
		}
	}

	logger.Infof("  ✓ Migrated %d users", len(usersList))
	return nil
}

// migrateShares migrates all shares from BoltDB to SQLite
func migrateShares(oldDB *storm.DB, sqlStore *sqldb.SQLStore) error {
	var sharesList []*share.Link
	err := oldDB.All(&sharesList)
	if err != nil {
		if err.Error() == "not found" {
			logger.Info("  No shares to migrate")
			return nil
		}
		return fmt.Errorf("failed to read shares from old DB: %w", err)
	}

	for _, link := range sharesList {
		err := sqlStore.SaveShare(link)
		if err != nil {
			return fmt.Errorf("failed to save share %s: %w", link.Hash, err)
		}
	}

	logger.Infof("  ✓ Migrated %d shares", len(sharesList))
	return nil
}

// migrateAccessRules migrates access rules and groups from BoltDB to SQLite
func migrateAccessRules(oldDB *storm.DB, sqlStore *sqldb.SQLStore) error {
	// Read the access rules JSON blob from BoltDB
	var data []byte
	err := oldDB.Get("access_rules", "rules", &data)
	if err != nil {
		if err.Error() == "not found" {
			logger.Info("  No access rules to migrate")
			return nil
		}
		return fmt.Errorf("failed to read access rules: %w", err)
	}

	// Unmarshal the data
	type dbStorage struct {
		AllRules      access.SourceRuleMap `json:"all_rules"`
		Groups        access.GroupMap      `json:"groups"`
		RevokedTokens map[string]struct{}  `json:"revoked_tokens"`
		HashedTokens  map[string]uint      `json:"hashed_tokens"`
	}

	var storage dbStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return fmt.Errorf("failed to unmarshal access rules: %w", err)
	}

	// Migrate access rules
	ruleCount := 0
	for source, rules := range storage.AllRules {
		for path, rule := range rules {
			err := sqlStore.SaveAccessRule(source, path, rule)
			if err != nil {
				return fmt.Errorf("failed to save access rule: %w", err)
			}
			ruleCount++
		}
	}
	logger.Infof("  ✓ Migrated %d access rules", ruleCount)

	// Migrate groups
	for name, members := range storage.Groups {
		err := sqlStore.SaveGroup(name, members)
		if err != nil {
			return fmt.Errorf("failed to save group %s: %w", name, err)
		}
	}
	logger.Infof("  ✓ Migrated %d groups", len(storage.Groups))

	// Migrate revoked tokens
	for tokenHash := range storage.RevokedTokens {
		err := sqlStore.SaveRevokedToken(tokenHash)
		if err != nil {
			return fmt.Errorf("failed to save revoked token: %w", err)
		}
	}
	logger.Infof("  ✓ Migrated %d revoked tokens", len(storage.RevokedTokens))

	// Migrate hashed tokens
	for tokenHash, userID := range storage.HashedTokens {
		err := sqlStore.SaveHashedToken(tokenHash, userID)
		if err != nil {
			return fmt.Errorf("failed to save hashed token: %w", err)
		}
	}
	logger.Infof("  ✓ Migrated %d hashed tokens", len(storage.HashedTokens))

	return nil
}

// migrateSettings migrates settings from BoltDB to SQLite
func migrateSettings(oldDB *storm.DB, sqlStore *sqldb.SQLStore) error {
	// Migrate main settings
	var set settings.Settings
	err := oldDB.Get("config", "settings", &set)
	if err != nil {
		if err.Error() != "not found" {
			// Try to read as raw JSON and transform database field
			var rawData map[string]interface{}
			if err2 := oldDB.Get("config", "settings", &rawData); err2 != nil {
				return fmt.Errorf("failed to read settings: %w", err)
			}

			// Handle database field schema change: string -> Database struct
			if rawData["server"] != nil {
				serverMap, ok := rawData["server"].(map[string]interface{})
				if ok && serverMap["database"] != nil {
					// Check if database is a string (old format)
					if dbStr, isString := serverMap["database"].(string); isString {
						// Convert string to Database struct format
						serverMap["database"] = map[string]interface{}{
							"path": dbStr,
						}
						serverMap["migrateFrom"] = dbStr
						logger.Info("  ✓ Converted database field from string to struct format")
					}
				}
			}

			// Marshal and unmarshal to validate
			data, err3 := json.Marshal(rawData)
			if err3 != nil {
				return fmt.Errorf("failed to marshal settings: %w", err3)
			}
			if err3 := json.Unmarshal(data, &set); err3 != nil {
				return fmt.Errorf("failed to unmarshal transformed settings: %w", err3)
			}
		}
	}

	if err == nil || err.Error() != "not found" {
		err = sqlStore.SaveSetting("settings", &set)
		if err != nil {
			return fmt.Errorf("failed to save settings: %w", err)
		}
		logger.Info("  ✓ Migrated main settings")
	}

	// Migrate server config
	var server settings.Server
	err = oldDB.Get("config", "server", &server)
	if err != nil {
		if err.Error() != "not found" {
			// Try to read as raw JSON and transform database field
			var rawData map[string]interface{}
			if err2 := oldDB.Get("config", "server", &rawData); err2 != nil {
				return fmt.Errorf("failed to read server config: %w", err)
			}

			// Handle database field schema change: string -> Database struct
			if rawData["database"] != nil {
				if dbStr, isString := rawData["database"].(string); isString {
					// Convert string to Database struct format
					rawData["database"] = map[string]interface{}{
						"path": dbStr,
					}
					rawData["migrateFrom"] = dbStr
					logger.Info("  ✓ Converted server database field from string to struct format")
				}
			}

			// Marshal and unmarshal to validate
			data, err3 := json.Marshal(rawData)
			if err3 != nil {
				return fmt.Errorf("failed to marshal server config: %w", err3)
			}
			if err3 := json.Unmarshal(data, &server); err3 != nil {
				return fmt.Errorf("failed to unmarshal transformed server config: %w", err3)
			}
		}
	}

	if err == nil || err.Error() != "not found" {
		err = sqlStore.SaveSetting("server", &server)
		if err != nil {
			return fmt.Errorf("failed to save server config: %w", err)
		}
		logger.Info("  ✓ Migrated server configuration")
	}

	// Migrate schema version
	var version int
	err = oldDB.Get("config", "version", &version)
	if err == nil {
		err = sqlStore.SaveSetting("bolt_schema_version", version)
		if err != nil {
			return fmt.Errorf("failed to save schema version: %w", err)
		}
		logger.Infof("  ✓ Migrated schema version: %d", version)
	}

	return nil
}

// migrateAuthMethods migrates auth methods from BoltDB to SQLite
func migrateAuthMethods(oldDB *storm.DB, sqlStore *sqldb.SQLStore) error {
	authTypes := []struct {
		name   string
		auther auth.Auther
	}{
		{"json", &auth.JSONAuth{}},
		{"proxy", &auth.ProxyAuth{}},
		{"none", &auth.NoAuth{}},
	}

	migratedCount := 0
	for _, at := range authTypes {
		err := oldDB.Get("config", "auther", at.auther)
		if err != nil {
			if err.Error() == "not found" {
				continue
			}
			logger.Warningf("  Failed to read auth method %s: %v", at.name, err)
			continue
		}

		err = sqlStore.SaveAuthMethod(at.name, at.auther)
		if err != nil {
			return fmt.Errorf("failed to save auth method %s: %w", at.name, err)
		}
		migratedCount++
	}

	logger.Infof("  ✓ Migrated %d auth methods", migratedCount)
	return nil
}

// migrateIndexInfo migrates index info from BoltDB to SQLite
func migrateIndexInfo(oldDB *storm.DB, sqlStore *sqldb.SQLStore) error {
	var indexList []*dbindex.IndexInfo
	err := oldDB.All(&indexList)
	if err != nil {
		if err.Error() == "not found" {
			logger.Info("  No index info to migrate")
			return nil
		}
		return fmt.Errorf("failed to read index info: %w", err)
	}

	for _, info := range indexList {
		err := sqlStore.SaveIndexInfo(info)
		if err != nil {
			return fmt.Errorf("failed to save index info for %s: %w", info.Path, err)
		}
	}

	logger.Infof("  ✓ Migrated %d index info entries", len(indexList))
	return nil
}
