package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	storm "github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
	"github.com/gtsteffaniak/go-logger/logger"
)

// checkMigrationNeeded is true when database.migrateFrom points at a BoltDB file to import and
// database.path (SQLite) does not exist yet or is empty.
func checkMigrationNeeded() bool {
	boltPath := settings.Config.Server.DatabaseV2.MigrateFrom
	if boltPath == "" {
		return false
	}
	if sqliteDatabasePopulated(settings.Config.Server.DatabaseV2.Path) {
		return false
	}
	return legacyBoltDatabasePopulated(boltPath)
}

// validateDatabasePaths enforces database path rules before opening or creating SQLite:
//  1. On a fresh install, fail if the configured SQLite path is database.db or an unrenamed
//     legacy Bolt file (database.db) is present in the working directory.
//  2. On a fresh install with database.migrateFrom set, fail if the legacy Bolt file is missing
//     or empty.
//  3. When SQLite already exists and database.migrateFrom is set, fail if the legacy Bolt file
//     is missing or empty.
//
// Otherwise the existing SQLite database is used, or a new one is created on first open.
func validateDatabasePaths() error {
	sqlitePath := settings.Config.Server.DatabaseV2.Path
	boltPath := settings.Config.Server.DatabaseV2.MigrateFrom
	sqliteExists := sqliteDatabasePopulated(sqlitePath)

	if !sqliteExists {
		if filepath.Base(sqlitePath) == "database.db" {
			return fmt.Errorf(
				"server.database path cannot be %q; use a different filename for the SQLite database (e.g. filebrowser.sqlite)",
				sqlitePath,
			)
		}
		if _, err := os.Stat("database.db"); err == nil {
			return fmt.Errorf(
				"old version of database file found at database.db, please rename it to database.db.old and set database.migrateFrom",
			)
		}
		if boltPath != "" {
			return legacyBoltDatabaseError(boltPath, sqlitePath, true)
		}
		return nil
	}

	if boltPath != "" {
		return legacyBoltDatabaseError(boltPath, sqlitePath, false)
	}
	return nil
}

func legacyBoltDatabaseError(boltPath, sqlitePath string, freshInstall bool) error {
	stat, err := os.Stat(boltPath)
	if err != nil {
		if os.IsNotExist(err) {
			if freshInstall {
				return fmt.Errorf(
					"database.migrateFrom is %q but that file does not exist and no SQLite database exists at %q",
					boltPath,
					sqlitePath,
				)
			}
			return fmt.Errorf(
				"database.migrateFrom is %q but that file does not exist; remove migrateFrom from config or restore the legacy database file",
				boltPath,
			)
		}
		return fmt.Errorf("database.migrateFrom is %q but cannot be read: %w", boltPath, err)
	}
	if stat.Size() == 0 {
		if freshInstall {
			return fmt.Errorf(
				"database.migrateFrom is %q but that file is empty and no SQLite database exists at %q",
				boltPath,
				sqlitePath,
			)
		}
		return fmt.Errorf(
			"database.migrateFrom is %q but that file is empty; remove migrateFrom from config or restore the legacy database file",
			boltPath,
		)
	}
	return nil
}

func sqliteDatabasePopulated(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.Size() > 0
}

func legacyBoltDatabasePopulated(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.Size() > 0
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

// normalizeUserScopesBeforeSQLite copies scopes from FrontendScopes into BackendScopes when the Bolt
// record only had JSON key "scopes" (FrontendUser) and nothing under backendScopes — CreateUser
// persists only BackendScopes in SQLite user_data.
func normalizeUserScopesBeforeSQLite(user *users.User) error {
	if len(user.BackendScopes) > 0 {
		return nil
	}
	backendScopes, err := users.APIScopesToBackend(user.FrontendScopes)
	if err != nil {
		return fmt.Errorf("convert frontend scopes to backend: %w", err)
	}
	for i := range backendScopes {
		backendScopes[i].Permissions = users.SourceFilePermissions{
			View:     true,
			Download: user.Permissions.Download,
			Modify:   user.Permissions.Modify,
			Delete:   user.Permissions.Delete,
			Create:   user.Permissions.Create,
		}
	}
	user.FrontendScopes = nil
	user.BackendScopes = backendScopes
	return nil
}

// migrateUsers migrates all users from BoltDB to SQLite. Each bolt user.ID is written as user_id
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

	promoted := 0
	for _, user := range usersList {
		logger.Debugf(
			"sidebar_migrate bolt_read user=%q count=%d links=%s",
			user.Username, len(user.SidebarLinks), usersidebar.FormatSidebarLinksForLog(user.SidebarLinks),
		)
		oldScopesCount := len(user.FrontendScopes)
		newScopesCount := len(user.BackendScopes)
		if err := normalizeUserScopesBeforeSQLite(user); err != nil {
			return fmt.Errorf("failed to normalize scopes for user %s: %w", user.Username, err)
		}
		if normalized, changed := usersidebar.NormalizeSidebarLinks(user.SidebarLinks); changed {
			user.SidebarLinks = normalized
		}
		logger.Debugf(
			"sidebar_migrate after_normalize user=%q backendScopes=%d sidebarCount=%d links=%s",
			user.Username, len(user.BackendScopes), len(user.SidebarLinks),
			usersidebar.FormatSidebarLinksForLog(user.SidebarLinks),
		)
		if len(user.BackendScopes) == 0 {
			settings.ApplyUserDefaults(user)
			logger.Debugf(
				"sidebar_migrate after_apply_defaults user=%q backendScopes=%d sidebarCount=%d links=%s",
				user.Username, len(user.BackendScopes), len(user.SidebarLinks),
				usersidebar.FormatSidebarLinksForLog(user.SidebarLinks),
			)
			if normalized, changed := usersidebar.NormalizeSidebarLinks(user.SidebarLinks); changed {
				user.SidebarLinks = normalized
			}
		}
		logger.Debugf(
			"sidebar_migrate after_normalize_links user=%q sidebarCount=%d links=%s",
			user.Username, len(user.SidebarLinks), usersidebar.FormatSidebarLinksForLog(user.SidebarLinks),
		)
		users.MigrateToSourcePermissions(user)
		normalizeUserTokensBeforeSQLite(user)
		updateTokens(user)
		if newScopesCount > oldScopesCount {
			promoted++
			logger.Infof("  user %q: Bolt had %d scopes, SQLite now has %d",
				user.Username, oldScopesCount, newScopesCount)
		}
		logger.Debugf(
			"sidebar_migrate before_sqlite_insert user=%q sidebarCount=%d links=%s",
			user.Username, len(user.SidebarLinks), usersidebar.FormatSidebarLinksForLog(user.SidebarLinks),
		)
		boltID := user.ID
		if err := sqlStore.CreateUser(user); err != nil {
			return fmt.Errorf("failed to save user %s (bolt id: %d): %w", user.Username, boltID, err)
		}
	}

	if promoted > 0 {
		logger.Infof("  Promoted frontend-only Bolt scopes to backend for %d user(s) before SQLite insert", promoted)
	}
	logger.Infof("  ✓ Migrated %d users", len(usersList))
	return nil
}

// normalizeUserTokensBeforeSQLite ensures Bolt name-keyed tokens have Name set so
// TokensForPersist retains them in SQLite user_data (Bolt often stores the name only as the map key).
func normalizeUserTokensBeforeSQLite(user *users.User) {
	if len(user.Tokens) == 0 {
		return
	}
	normalized := make(map[string]users.AuthToken)
	for key, token := range user.Tokens {
		name := token.Name
		if name == "" {
			if key == token.Token {
				continue
			}
			name = key
		}
		if _, exists := normalized[name]; exists {
			continue
		}
		token.Name = name
		if token.Token == "" {
			token.Token = token.Key
		}
		token.Permissions = users.SanitizeTokenPermissions(token.Permissions)
		users.StoreToken(normalized, token)
	}
	user.Tokens = normalized
}

// migrateShares migrates all shares from BoltDB to SQLite.
//
// Legacy Bolt "Link" / "Share" JSON (flat CommonShare + Link fields):
//   - "source"  → Share.SourcePath (backend source path; not the share item path)
//   - "path"    → Share.Path (index-relative path within that source)
//   - "userID"  → Share.UserID unchanged (same numeric id as migrated users in SQLite)
//   - "password_hash" → Share.PasswordHash (username is never stored on the share row)
//
// New users created after migration get random uint64 ids (utils.RandomUint64ID); Bolt ids 1,2,3…
// remain valid. PrepForFrontend resolves Share.UserID to username for API responses only.
func migrateShares(oldDB *storm.DB, sqlStore *sqldb.SQLStore) error {
	boltShareBuckets := []string{"Share", "Link"}
	var sharesList []*share.LegacyShare
	for _, bucket := range boltShareBuckets {
		var batch []*share.LegacyShare
		err := oldDB.Select().Bucket(bucket).Find(&batch)
		if err != nil && err != storm.ErrNotFound {
			return fmt.Errorf("failed to read shares from old DB bucket %q: %w", bucket, err)
		}
		if len(batch) > 0 {
			sharesList = append(sharesList, batch...)
		}
	}

	if len(sharesList) == 0 {
		logger.Info("  No shares to migrate")
		return nil
	}

	for _, legacy := range sharesList {
		link := legacy.ToShare()
		if link.UserID == 0 {
			return fmt.Errorf("failed to save share %s: owner user id is missing", link.Hash)
		}
		// Legacy Bolt "source" → SourcePath; normalize when it was stored as a display name.
		if link.SourcePath != "" {
			if _, ok := settings.Config.Server.SourceMap[link.SourcePath]; !ok {
				if src, ok := settings.Config.Server.NameToSource[link.SourcePath]; ok {
					link.SourceName = src.Name
					link.SourcePath = src.Path
				}
			} else if link.SourceName == "" {
				link.SourceName = settings.Config.Server.SourceMap[link.SourcePath].Name
			}
		}
		// Do not persist API-only fields; PrepForFrontend fills these on read.
		link.ShareURL = ""
		link.DownloadURL = ""
		link.FrontendShareInfo.SourceURL = ""
		link.CanEditShare = false
		if err := sqlStore.SaveShare(&link); err != nil {
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
