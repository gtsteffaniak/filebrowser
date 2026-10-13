package sql

import (
	"fmt"

	"github.com/gtsteffaniak/go-logger/logger"
)

// indexSchemaVersion is bumped whenever the index_items/search schema changes; a
// mismatch causes the (rebuildable) index database to be recreated and rescanned.
const indexSchemaVersion = 1

// supportsFTS5 reports whether the driver can create an FTS5 table with the trigram
// tokenizer (SQLite >= 3.34 with FTS5 compiled in).
func (db *IndexDB) supportsFTS5() bool {
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS temp.__fts_probe USING fts5(x, tokenize='trigram')`); err != nil {
		return false
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS temp.__fts_probe`)
	return true
}

// ensureSearchIndex creates the FTS5 trigram search table and its sync triggers.
// When the driver lacks FTS5/trigram support it logs once and leaves FTS disabled so
// search falls back to the unindexed path.
func (db *IndexDB) ensureSearchIndex() {
	if !db.supportsFTS5() {
		logger.Errorf("SQLite FTS5 trigram support is unavailable in this build; search will use the unindexed fallback. For cgo builds use -tags \"cgosql sqlite_fts5\".")
		db.ftsReady.Store(false)
		return
	}
	const ddl = `
	CREATE VIRTUAL TABLE IF NOT EXISTS index_items_fts USING fts5(
		name, spath,
		content='index_items', content_rowid='rowid',
		tokenize='trigram', detail=none
	);
	CREATE TRIGGER IF NOT EXISTS index_items_fts_ai AFTER INSERT ON index_items BEGIN
		INSERT INTO index_items_fts(rowid, name, spath) VALUES (new.rowid, new.name, new.spath);
	END;
	CREATE TRIGGER IF NOT EXISTS index_items_fts_ad AFTER DELETE ON index_items BEGIN
		INSERT INTO index_items_fts(index_items_fts, rowid, name, spath) VALUES ('delete', old.rowid, old.name, old.spath);
	END;
	CREATE TRIGGER IF NOT EXISTS index_items_fts_au AFTER UPDATE OF name ON index_items
	WHEN old.name IS NOT new.name BEGIN
		INSERT INTO index_items_fts(index_items_fts, rowid, name, spath) VALUES ('delete', old.rowid, old.name, old.spath);
		INSERT INTO index_items_fts(rowid, name, spath) VALUES (new.rowid, new.name, new.spath);
	END;
	`
	if _, err := db.Exec(ddl); err != nil {
		logger.Errorf("failed to create the FTS search index (%v); search will use the unindexed fallback", err)
		db.ftsReady.Store(false)
		return
	}
	db.ftsReady.Store(true)
}

// FTSReady reports whether the trigram search index is available for queries.
func (db *IndexDB) FTSReady() bool {
	return db.ftsReady.Load()
}

// checkIndexSchemaVersion verifies a reused database matches the current schema and
// has a healthy FTS index. A non-nil error means the database should be recreated.
func (db *IndexDB) checkIndexSchemaVersion() error {
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}
	if version != indexSchemaVersion {
		return fmt.Errorf("schema version %d, want %d", version, indexSchemaVersion)
	}
	var hasFTS int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'index_items_fts'`).Scan(&hasFTS); err != nil {
		return fmt.Errorf("check fts table: %w", err)
	}
	if hasFTS == 0 {
		return fmt.Errorf("search index missing")
	}
	return db.checkFTSHealth()
}

// checkFTSHealth compares the FTS index against the base table and runs a cheap
// internal consistency check. The full content check (rank=1) is far slower and is
// skipped.
func (db *IndexDB) checkFTSHealth() error {
	var items, docs int
	if err := db.QueryRow(`SELECT count(*) FROM index_items`).Scan(&items); err != nil {
		return fmt.Errorf("count index_items: %w", err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM index_items_fts_docsize`).Scan(&docs); err != nil {
		return fmt.Errorf("count fts docsize: %w", err)
	}
	if items != docs {
		return fmt.Errorf("search index row mismatch: index_items=%d fts=%d", items, docs)
	}
	if _, err := db.Exec(`INSERT INTO index_items_fts(index_items_fts, rank) VALUES('integrity-check', 0)`); err != nil {
		return fmt.Errorf("fts integrity check: %w", err)
	}
	return nil
}

// optimizeFTS merges the FTS index segments; call after bulk deletes or heavy churn.
func (db *IndexDB) optimizeFTS() {
	if !db.FTSReady() {
		return
	}
	if _, err := db.Exec(`INSERT INTO index_items_fts(index_items_fts) VALUES('optimize')`); err != nil {
		logger.Warningf("FTS optimize failed: %v", err)
	}
}
