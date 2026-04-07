package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// checkMigrationNeeded is true when database.migrateFrom points at a BoltDB file to import and
// database.path (SQLite) does not exist yet or is empty.
func checkMigrationNeeded() bool {
	boltPath := settings.Config.Server.DatabaseV2.MigrateFrom
	sqlitePath := settings.Config.Server.DatabaseV2.Path
	if boltPath == "" {
		return false
	}
	boltOK := false
	if stat, err := os.Stat(boltPath); err == nil && stat.Size() > 0 {
		boltOK = true
	}
	sqliteExists := false
	if stat, err := os.Stat(sqlitePath); err == nil && stat.Size() > 0 {
		sqliteExists = true
	}
	return boltOK && !sqliteExists
}

// migrateFromBoltToSQLite migrates essential data from BoltDB to SQLite
func migrateFromBoltToSQLite() error {
	oldDBPath := settings.Config.Server.DatabaseV2.MigrateFrom
	newDBPath := settings.Config.Server.DatabaseV2.Path

	logger.Info("========================================")
	logger.Info("Starting migration from BoltDB to SQLite")
	logger.Info("========================================")
	// Open old BoltDB (read-only)
	logger.Info("Opening old BoltDB...")
	oldDB, err := storm.Open(oldDBPath)
	if err != nil {
		return fmt.Errorf("failed to open old database: %w", err)
	}
	logger.Info("✓ Old database opened")

	// Initialize new SQLite database
	logger.Info("Initializing new SQLite database...")
	sqlStore, _, err := sqldb.NewSQLStoreWithOptions(newDBPath, sqldb.NewSQLStoreOpts{SkipQuickSetup: true})
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

	// Migrate users (bolt User.ID is stored as SQLite users.user_id)
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
	logger.Infof("Your old BoltDB file is unchanged at: %s", oldDBPath)
	logger.Infof("New SQLite database created at: %s", newDBPath)

	return nil
}

// migrateUsers migrates all users from BoltDB to SQLite. Each bolt user.ID is written as user_id
// (CreateUser keeps a non-zero ID).
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
		boltID := user.ID
		if err := sqlStore.CreateUser(user); err != nil {
			return fmt.Errorf("failed to save user %s (bolt id: %d): %w", user.Username, boltID, err)
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
		if link.UserID == 0 {
			return fmt.Errorf("failed to save share %s: owner user id is missing", link.Hash)
		}
		err := sqlStore.SaveShare(link)
		if err != nil {
			return fmt.Errorf("failed to save share %s: %w", link.Hash, err)
		}
	}

	logger.Infof("  ✓ Migrated %d shares", len(sharesList))
	return nil
}

// migrateAccessRules migrates access control data from BoltDB to SQLite.
// Path rules (AllRules) and group definitions (Groups) are unchanged through migration
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

	// Migrate hashed tokens (bolt stored owner user id)
	for tokenHash, userID := range storage.HashedTokens {
		if userID == 0 {
			logger.Warningf("  skipping hashed token: invalid user id 0")
			continue
		}
		err := sqlStore.SaveHashedToken(tokenHash, uint64(userID))
		if err != nil {
			return fmt.Errorf("failed to save hashed token: %w", err)
		}
	}
	logger.Infof("  ✓ Migrated %d hashed tokens", len(storage.HashedTokens))

	return nil
}

// migrateIndexInfo migrates index info metadata from BoltDB to SQLite
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
