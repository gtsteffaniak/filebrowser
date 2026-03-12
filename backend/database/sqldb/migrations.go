package sqldb

import (
	"database/sql"
	"fmt"
)

const currentSchemaVersion = 1

// Schema creates all tables for the SQLite database
func createSchema(db *sql.DB) error {
	schema := `
	-- Schema version tracking
	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		updated_at INTEGER NOT NULL
	);

	-- Users table
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		perm_api BOOLEAN NOT NULL DEFAULT 0,
		perm_admin BOOLEAN NOT NULL DEFAULT 0,
		perm_modify BOOLEAN NOT NULL DEFAULT 0,
		perm_share BOOLEAN NOT NULL DEFAULT 0,
		perm_realtime BOOLEAN NOT NULL DEFAULT 0,
		perm_delete BOOLEAN NOT NULL DEFAULT 0,
		perm_create BOOLEAN NOT NULL DEFAULT 0,
		perm_download BOOLEAN NOT NULL DEFAULT 1,
		user_data TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_admin ON users(perm_admin);
	CREATE INDEX IF NOT EXISTS idx_users_api ON users(perm_api);

	-- Shares table
	CREATE TABLE IF NOT EXISTS shares (
		hash TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
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

	-- Hashed tokens table
	CREATE TABLE IF NOT EXISTS hashed_tokens (
		token_hash TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL
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

// runMigrations applies any necessary schema migrations
func runMigrations(db *sql.DB, fromVersion int) error {
	// Future migrations would go here
	// For now, we only have version 1
	if fromVersion < currentSchemaVersion {
		// Apply migrations sequentially
		for v := fromVersion + 1; v <= currentSchemaVersion; v++ {
			switch v {
			case 1:
				// Version 1 is the initial schema, already created
				continue
			default:
				return fmt.Errorf("unknown schema version: %d", v)
			}
		}

		// Update schema version
		_, err := db.Exec("UPDATE schema_version SET version = ?, updated_at = ?",
			currentSchemaVersion, currentTimestamp())
		if err != nil {
			return fmt.Errorf("failed to update schema version: %w", err)
		}
	}

	return nil
}
