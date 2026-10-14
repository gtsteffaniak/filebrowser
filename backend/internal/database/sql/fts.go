package sql

import (
	"fmt"

	"github.com/gtsteffaniak/go-logger/logger"
)

// supportsFTS5 reports whether the driver can create an FTS5 table with the trigram
// tokenizer (SQLite >= 3.34 with FTS5 compiled in).
func (db *IndexDB) supportsFTS5() bool {
	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS temp.__fts_probe USING fts5(x, tokenize='trigram')`); err != nil {
		return false
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS temp.__fts_probe`)
	return true
}

// ftsTableExists reports whether the FTS search table is present.
func (db *IndexDB) ftsTableExists() bool {
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'index_items_fts'`).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// FTSReady reports whether the trigram search index is available for queries.
func (db *IndexDB) FTSReady() bool {
	return db.ftsReady.Load()
}

const ftsDDL = `
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

// syncSearchIndex creates the FTS5 trigram index when enabled and drops it when
// disabled. The index is derived data: enabling it builds from existing rows (no
// rescan), and disabling it removes the table and triggers.
func (db *IndexDB) syncSearchIndex(enabled bool) {
	if !enabled {
		db.dropSearchIndex()
		return
	}
	if !db.supportsFTS5() {
		logger.Errorf("indexSqlConfig.fts5 is enabled but FTS5 trigram support is unavailable in this build; search will use the unindexed fallback. For cgo builds use -tags \"cgosql sqlite_fts5\".")
		db.ftsReady.Store(false)
		return
	}
	if err := db.ensureSpathColumn(); err != nil {
		logger.Errorf("failed to add the spath column (%v); search will use the unindexed fallback", err)
		db.ftsReady.Store(false)
		return
	}
	existed := db.ftsTableExists()
	if _, err := db.Exec(ftsDDL); err != nil {
		logger.Errorf("failed to create the FTS search index (%v); search will use the unindexed fallback", err)
		db.ftsReady.Store(false)
		return
	}
	if !existed {
		if _, err := db.Exec(`INSERT INTO index_items_fts(index_items_fts) VALUES('rebuild')`); err != nil {
			logger.Errorf("failed to populate the FTS search index (%v); search will use the unindexed fallback", err)
			db.ftsReady.Store(false)
			return
		}
	}
	db.ftsReady.Store(true)
}

// dropSearchIndex removes the FTS table and its triggers (used when the feature is
// disabled or the index needs a rebuild).
func (db *IndexDB) dropSearchIndex() {
	db.ftsReady.Store(false)
	if !db.ftsTableExists() {
		return
	}
	if _, err := db.Exec(`
		DROP TRIGGER IF EXISTS index_items_fts_ai;
		DROP TRIGGER IF EXISTS index_items_fts_ad;
		DROP TRIGGER IF EXISTS index_items_fts_au;
		DROP TABLE IF EXISTS index_items_fts;
	`); err != nil {
		logger.Warningf("failed to drop the FTS search index: %v", err)
	}
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
