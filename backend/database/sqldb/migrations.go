package sqldb

import (
	"database/sql"
	"fmt"

	"github.com/gtsteffaniak/go-logger/logger"
)

// currentSchemaVersion 2: shares and hashed_tokens key owner by user_id (decimal text), not username.
const currentSchemaVersion = 2

// Schema creates all tables for the SQLite database
func createSchema(db *sql.DB) error {
	schema := `
	-- Schema version tracking
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		updated_at INTEGER NOT NULL
	);

	-- Users: username is the primary key; user_id is stable uint64 (decimal text)
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY NOT NULL,
		user_id TEXT NOT NULL,
		perm_api INTEGER NOT NULL DEFAULT 0,
		perm_admin INTEGER NOT NULL DEFAULT 0,
		perm_realtime INTEGER NOT NULL DEFAULT 0,
		user_data TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_users_user_id ON users(user_id);
	CREATE UNIQUE INDEX IF NOT EXISTS idx_users_user_id_unique ON users(user_id);
	CREATE INDEX IF NOT EXISTS idx_users_admin ON users(perm_admin);
	CREATE INDEX IF NOT EXISTS idx_users_api ON users(perm_api);

	-- Shares (owner is user_id)
	CREATE TABLE IF NOT EXISTS shares (
		hash TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		source TEXT NOT NULL,
		path TEXT NOT NULL,
		expire INTEGER NOT NULL DEFAULT 0,
		downloads INTEGER NOT NULL DEFAULT 0,
		password_hash TEXT,
		token TEXT,
		user_downloads TEXT,
		share_settings TEXT NOT NULL,
		version INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX IF NOT EXISTS idx_shares_user_id ON shares(user_id);
	CREATE INDEX IF NOT EXISTS idx_shares_source ON shares(source);
	CREATE INDEX IF NOT EXISTS idx_shares_path ON shares(path);
	CREATE INDEX IF NOT EXISTS idx_shares_expire ON shares(expire);
	CREATE INDEX IF NOT EXISTS idx_shares_source_path ON shares(source, path);

	-- Access rules table
	CREATE TABLE IF NOT EXISTS access_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		path TEXT NOT NULL,
		rule_data TEXT NOT NULL,
		UNIQUE(source, path)
	);
	CREATE INDEX IF NOT EXISTS idx_access_rules_source ON access_rules(source);
	CREATE INDEX IF NOT EXISTS idx_access_rules_path ON access_rules(path);
	CREATE INDEX IF NOT EXISTS idx_access_rules_source_path ON access_rules(source, path);

	-- Groups table
	CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE NOT NULL,
		members TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);

	-- Revoked tokens table
	CREATE TABLE IF NOT EXISTS revoked_tokens (
		token_hash TEXT PRIMARY KEY,
		revoked_at INTEGER NOT NULL
	);

	-- Hashed tokens (minimal JWT → owner user_id)
	CREATE TABLE IF NOT EXISTS hashed_tokens (
		token_hash TEXT PRIMARY KEY,
		user_id TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_hashed_tokens_user_id ON hashed_tokens(user_id);

	-- Index info table
	CREATE TABLE IF NOT EXISTS index_info (
		path TEXT PRIMARY KEY,
		source TEXT NOT NULL,
		complexity INTEGER NOT NULL DEFAULT 0,
		num_dirs INTEGER NOT NULL DEFAULT 0,
		num_files INTEGER NOT NULL DEFAULT 0,
		scanners TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_index_info_source ON index_info(source);

	-- Auth methods table
	CREATE TABLE IF NOT EXISTS auth_methods (
		type TEXT PRIMARY KEY,
		config TEXT NOT NULL
	);

	-- Settings table
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	`

	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// initializeSchemaVersion sets the initial schema version
func initializeSchemaVersion(db *sql.DB) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check schema version: %w", err)
	}

	if count == 0 {
		_, err = db.Exec("INSERT INTO schema_version (version, updated_at) VALUES (?, ?)",
			currentSchemaVersion, currentTimestamp())
		if err != nil {
			return fmt.Errorf("failed to initialize schema version: %w", err)
		}
	}

	return nil
}

// getSchemaVersion returns the current schema version from the database
func getSchemaVersion(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow("SELECT version FROM schema_version ORDER BY version DESC LIMIT 1").Scan(&version)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get schema version: %w", err)
	}
	return version, nil
}

func runMigrations(db *sql.DB, fromVersion int) error {
	if fromVersion > currentSchemaVersion {
		_, err := db.Exec("UPDATE schema_version SET version = ?, updated_at = ?",
			currentSchemaVersion, currentTimestamp())
		if err != nil {
			return fmt.Errorf("failed to normalize schema version: %w", err)
		}
		return nil
	}
	if fromVersion >= currentSchemaVersion {
		return nil
	}

	for v := fromVersion + 1; v <= currentSchemaVersion; v++ {
		switch v {
		case 1:
			// Legacy placeholder (initial releases used createSchema only).
		case 2:
			if err := migrateSQLiteV1ToV2OwnerIDs(db); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown schema version: %d", v)
		}
	}

	_, err := db.Exec("UPDATE schema_version SET version = ?, updated_at = ?",
		currentSchemaVersion, currentTimestamp())
	if err != nil {
		return fmt.Errorf("failed to update schema version: %w", err)
	}

	return nil
}

// migrateSQLiteV1ToV2OwnerIDs rewrites shares.username and hashed_tokens.username → user_id via users.user_id.
func migrateSQLiteV1ToV2OwnerIDs(db *sql.DB) error {
	if err := migrateSharesUsernameToUserID(db); err != nil {
		return err
	}
	return migrateHashedTokensUsernameToUserID(db)
}

func migrateSharesUsernameToUserID(db *sql.DB) error {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('shares') WHERE name = 'user_id'`).Scan(&n)
	if err != nil {
		return fmt.Errorf("inspect shares.user_id: %w", err)
	}
	if n > 0 {
		return nil
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('shares') WHERE name = 'username'`).Scan(&n)
	if err != nil {
		return fmt.Errorf("inspect shares.username: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("shares table has neither user_id nor username")
	}

	logger.Infof("SQLite migration v2: shares.username → shares.user_id")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
CREATE TABLE shares__v2 (
	hash TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	source TEXT NOT NULL,
	path TEXT NOT NULL,
	expire INTEGER NOT NULL DEFAULT 0,
	downloads INTEGER NOT NULL DEFAULT 0,
	password_hash TEXT,
	token TEXT,
	user_downloads TEXT,
	share_settings TEXT NOT NULL,
	version INTEGER NOT NULL DEFAULT 0
)`)
	if err != nil {
		return fmt.Errorf("create shares__v2: %w", err)
	}
	_, err = tx.Exec(`
INSERT INTO shares__v2 (hash, user_id, source, path, expire, downloads, password_hash, token, user_downloads, share_settings, version)
SELECT s.hash, u.user_id, s.source, s.path, s.expire, s.downloads, s.password_hash, s.token, s.user_downloads, s.share_settings, s.version
FROM shares s INNER JOIN users u ON u.username = s.username`)
	if err != nil {
		return fmt.Errorf("copy shares to v2: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE shares`); err != nil {
		return fmt.Errorf("drop old shares: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE shares__v2 RENAME TO shares`); err != nil {
		return fmt.Errorf("rename shares__v2: %w", err)
	}
	for _, stmt := range []string{
		`CREATE INDEX IF NOT EXISTS idx_shares_user_id ON shares(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_shares_source ON shares(source)`,
		`CREATE INDEX IF NOT EXISTS idx_shares_path ON shares(path)`,
		`CREATE INDEX IF NOT EXISTS idx_shares_expire ON shares(expire)`,
		`CREATE INDEX IF NOT EXISTS idx_shares_source_path ON shares(source, path)`,
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("recreate share indexes: %w", err)
		}
	}
	return tx.Commit()
}

func migrateHashedTokensUsernameToUserID(db *sql.DB) error {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('hashed_tokens') WHERE name = 'user_id'`).Scan(&n)
	if err != nil {
		return fmt.Errorf("inspect hashed_tokens.user_id: %w", err)
	}
	if n > 0 {
		return nil
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('hashed_tokens') WHERE name = 'username'`).Scan(&n)
	if err != nil {
		return fmt.Errorf("inspect hashed_tokens.username: %w", err)
	}
	if n == 0 {
		return nil
	}

	logger.Infof("SQLite migration v2: hashed_tokens.username → hashed_tokens.user_id")
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
CREATE TABLE hashed_tokens__v2 (
	token_hash TEXT PRIMARY KEY,
	user_id TEXT NOT NULL
)`)
	if err != nil {
		return fmt.Errorf("create hashed_tokens__v2: %w", err)
	}
	_, err = tx.Exec(`
INSERT INTO hashed_tokens__v2 (token_hash, user_id)
SELECT ht.token_hash, u.user_id FROM hashed_tokens ht INNER JOIN users u ON u.username = ht.username`)
	if err != nil {
		return fmt.Errorf("copy hashed_tokens to v2: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE hashed_tokens`); err != nil {
		return fmt.Errorf("drop old hashed_tokens: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE hashed_tokens__v2 RENAME TO hashed_tokens`); err != nil {
		return fmt.Errorf("rename hashed_tokens__v2: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_hashed_tokens_user_id ON hashed_tokens(user_id)`); err != nil {
		return fmt.Errorf("recreate hashed_tokens index: %w", err)
	}
	return tx.Commit()
}
