package sqldb

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestMigrationAddsHashedTokenSessionColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-tokens.db")
	store, _, err := NewSQLStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// Recreate the pre-v3 hashed_tokens table without is_session.
	if _, err = store.db.Exec(`DROP TABLE hashed_tokens`); err != nil {
		t.Fatalf("drop hashed_tokens: %v", err)
	}
	if _, err = store.db.Exec(`CREATE TABLE hashed_tokens (token_hash TEXT PRIMARY KEY, user_id TEXT NOT NULL)`); err != nil {
		t.Fatalf("create legacy hashed_tokens: %v", err)
	}
	if _, err = store.db.Exec(`INSERT INTO hashed_tokens (token_hash, user_id) VALUES ('legacy-hash', '7')`); err != nil {
		t.Fatalf("insert legacy hashed token: %v", err)
	}
	if _, err = store.db.Exec(`UPDATE schema_version SET version = 2`); err != nil {
		t.Fatalf("set schema version: %v", err)
	}

	if err = runMigrations(store.db, 2); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	if err = store.SaveHashedToken("new-hash", 8, true); err != nil {
		t.Fatalf("SaveHashedToken after migration: %v", err)
	}
	records, err := store.GetAllHashedTokens()
	if err != nil {
		t.Fatalf("GetAllHashedTokens: %v", err)
	}
	legacy, ok := records["legacy-hash"]
	if !ok || legacy.UserID != 7 {
		t.Fatalf("legacy mapping = %+v, ok=%v; want user 7", legacy, ok)
	}
	if legacy.IsSession {
		t.Fatal("legacy rows must migrate as non-session tokens")
	}
	if !records["new-hash"].IsSession {
		t.Fatal("session flag was not persisted")
	}
}

func TestGetSettingMissingReturnsSentinel(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "settings.db")
	store, _, err := NewSQLStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if _, err := store.GetSetting("does.not.exist"); !errors.Is(err, ErrSettingNotFound) {
		t.Fatalf("GetSetting missing error = %v, want ErrSettingNotFound", err)
	}
}
