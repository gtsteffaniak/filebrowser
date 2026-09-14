package sqldb

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
)

func createLegacySharesDB(t *testing.T) *SQLStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "legacy-shares.db")
	store, _, err := NewSQLStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}

	_, err = store.db.Exec(`DROP TABLE shares`)
	if err != nil {
		t.Fatalf("drop shares: %v", err)
	}
	_, err = store.db.Exec(`
		CREATE TABLE shares (
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
		t.Fatalf("create legacy shares table: %v", err)
	}
	_, err = store.db.Exec(`
		INSERT INTO shares (
			hash, user_id, source, path, expire, downloads, password_hash, token,
			user_downloads, share_settings, version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"legacy-null-token",
		"1",
		"/data",
		"/docs/file.txt",
		0,
		0,
		"",
		nil,
		"{}",
		`{"shareType":"normal"}`,
		1,
	)
	if err != nil {
		t.Fatalf("insert legacy share: %v", err)
	}

	return store
}

func scanLegacyShareToken(t *testing.T, store *SQLStore, hash string) string {
	t.Helper()
	var token string
	err := store.db.QueryRow(`
		SELECT hash, user_id, source, path, expire, downloads,
		       password_hash, token, user_downloads, share_settings, version
		FROM shares WHERE hash = ?`, hash).Scan(
		new(string),
		new(string),
		new(string),
		new(string),
		new(int64),
		new(int),
		new(string),
		&token,
		new([]byte),
		new([]byte),
		new(int),
	)
	if err != nil {
		t.Fatalf("scan legacy share row: %v", err)
	}
	return token
}

func TestMigrationNormalizesNullLegacyShareToken(t *testing.T) {
	store := createLegacySharesDB(t)

	if err := runMigrations(store.db, 1); err != nil {
		t.Fatalf("runMigrations: %v", err)
	}

	token := scanLegacyShareToken(t, store, "legacy-null-token")
	if token != "" {
		t.Fatalf("token = %q, want empty string after migration", token)
	}

	links, err := store.ListAllShares()
	if err != nil {
		t.Fatalf("ListAllShares: %v", err)
	}
	if len(links) != 1 || links[0].Hash != "legacy-null-token" {
		t.Fatalf("unexpected shares loaded: %#v", links)
	}
}

func TestSaveSharePreservesLegacyTokenColumn(t *testing.T) {
	store := createLegacySharesDB(t)
	if err := normalizeLegacyShareTokens(store.db); err != nil {
		t.Fatalf("normalizeLegacyShareTokens: %v", err)
	}

	_, err := store.db.Exec(`UPDATE shares SET token = ? WHERE hash = ?`, "keep-me", "legacy-null-token")
	if err != nil {
		t.Fatalf("seed legacy token: %v", err)
	}

	link, err := store.GetShareByHash("legacy-null-token")
	if err != nil {
		t.Fatalf("GetShareByHash: %v", err)
	}
	link.Downloads = 2

	if err := store.SaveShare(link); err != nil {
		t.Fatalf("SaveShare: %v", err)
	}

	token := scanLegacyShareToken(t, store, "legacy-null-token")
	if token != "keep-me" {
		t.Fatalf("token = %q, want keep-me", token)
	}

	var tokenSQL sql.NullString
	if err := store.db.QueryRow(`SELECT token FROM shares WHERE hash = ?`, "legacy-null-token").Scan(&tokenSQL); err != nil {
		t.Fatalf("select token: %v", err)
	}
	if !tokenSQL.Valid {
		t.Fatal("expected legacy token column to remain non-null after save")
	}
}

func TestSaveShareSetsEmptyLegacyTokenForNewShare(t *testing.T) {
	store := createLegacySharesDB(t)
	if err := normalizeLegacyShareTokens(store.db); err != nil {
		t.Fatalf("normalizeLegacyShareTokens: %v", err)
	}

	newShare := &share.Share{
		ShareSettings: share.ShareSettings{
			FrontendShareInfo: share.FrontendShareInfo{ShareType: "normal"},
		},
		ShareColumns: share.ShareColumns{
			Hash: "brand-new-share",
			Path: "/new/file.txt",
		},
		SourcePath: "/data",
		UserID:     1,
		Version:    1,
	}
	if err := store.SaveShare(newShare); err != nil {
		t.Fatalf("SaveShare: %v", err)
	}

	token := scanLegacyShareToken(t, store, "brand-new-share")
	if token != "" {
		t.Fatalf("token = %q, want empty string for new share", token)
	}
}
