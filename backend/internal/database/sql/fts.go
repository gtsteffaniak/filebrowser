package sql

import (
	"database/sql"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

// CreateFullTextTable creates the FTS5 table that supports content search.
// It is separate from CreateIndexTable so a build without FTS5 support can keep
// the name-only index working (fullTextDisabled).
func (db *IndexDB) CreateFullTextTable() error {
	if db.fullTextDisabled {
		return nil
	}
	db.fullTextMu.Lock()
	defer db.fullTextMu.Unlock()
	// rowid is kept in sync with index_items.rowid for O(log n) joins and prefix deletes.
	// Bodies are rewritten wholesale on each upsert, so deletes never need the old text.
	_, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS index_fts USING fts5(
		source UNINDEXED,
		path UNINDEXED,
		body,
		tokenize = 'unicode61 remove_diacritics 2'
	)`)
	if err != nil {
		logger.Warningf("[DB] full-text search disabled, FTS5 unavailable: %v", err)
		db.fullTextDisabled = true
		return nil
	}
	return nil
}

// SearchFullText returns index paths for source whose body matches the terms
// (prefix matching; AND when matchAll is set, OR otherwise).
// scope is a directory index path ending in "/".
// cursorPath/cursorLen page past an earlier result: paths ordered after the
// (length, path) cursor key are returned, including equal-length paths after it.
func (db *IndexDB) SearchFullText(source, scope string, terms []string, matchAll bool, limit int, cursorPath string, cursorLen int) ([]string, error) {
	expr := fullTextMatchExpr(terms, matchAll)
	if db.fullTextDisabled || expr == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	// index_items is not joined here: doing so made the planner scan index_fts
	// instead of using the FTS index; scope filtering is a cheap GLOB on the FTS rows.
	nameGlob := sqliteGlobEscape(scope) + "*"
	// Order by path length so LIMIT keeps the shallowest matches, matching the
	// caller's final result ordering; the cursor key is (length(path), path).
	rows, err := db.Query(`
		SELECT path FROM index_fts
		WHERE body MATCH ? AND source = ? AND path GLOB ?
		AND (length(path) > ? OR (length(path) = ? AND path > ?))
		ORDER BY length(path), path
		LIMIT ?`, expr, source, nameGlob, cursorLen, cursorLen, cursorPath, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	paths := make([]string, 0, limit)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

// fullTextMatchExpr builds an FTS5 MATCH expression from user terms. Each term is
// quoted (FTS syntax in user input stays literal) and prefix-matched.
func fullTextMatchExpr(terms []string, matchAll bool) string {
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		parts = append(parts, `"`+strings.ReplaceAll(t, `"`, `""`)+`"*`)
	}
	join := " OR "
	if matchAll {
		join = " AND "
	}
	return strings.Join(parts, join)
}

// sqliteGlobEscape escapes GLOB metacharacters so a literal path prefix stays literal.
func sqliteGlobEscape(s string) string {
	return strings.NewReplacer("*", "[*]", "?", "[?]", "[", "[[]").Replace(s)
}

// upsertFullText replaces the content row for one freshly upserted index_items row.
// The old row is dropped first so an upsert without a body never leaves stale text searchable.
// The rowid is looked up by (source, path): on ON CONFLICT DO UPDATE SQLite does not insert,
// so LastInsertId can still point at an earlier row.
func upsertFullText(tx *sql.Tx, stmt *sql.Stmt, source string, info *iteminfo.FileInfo) error {
	empty := info.FullTextBody == nil || *info.FullTextBody == ""
	var rowid int64
	if err := tx.QueryRow(`SELECT rowid FROM index_items WHERE source = ? AND path = ?`, source, info.Path).Scan(&rowid); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM index_fts WHERE rowid = ?`, rowid); err != nil {
		return err
	}
	if empty {
		return nil
	}
	_, err := stmt.Exec(rowid, source, info.Path, *info.FullTextBody)
	return err
}

// deleteItemRowsTx deletes matching index_items and index_fts rows in one transaction so a
// concurrent insert cannot interleave between the two deletes.
func (db *IndexDB) deleteItemRowsTx(whereClause string, args ...interface{}) error {
	tx, err := db.BeginTransaction()
	if err != nil {
		// BeginTransaction locks mu before Begin and does not unlock on error.
		db.mu.Unlock()
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
		db.mu.Unlock()
	}()
	if _, err := tx.Exec("DELETE FROM index_items WHERE "+whereClause, args...); err != nil {
		return err
	}
	if !db.fullTextOff() {
		if _, err := tx.Exec("DELETE FROM index_fts WHERE "+whereClause, args...); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// fullTextOff reports whether full-text rows are unavailable or disabled by config.
func (db *IndexDB) fullTextOff() bool {
	db.fullTextMu.Lock()
	defer db.fullTextMu.Unlock()
	return db.fullTextDisabled
}

// deleteFullText removes full-text rows matching whereClause (same args as the index_items delete).
// Newly re-indexed items get their row back on the next scan.
func (db *IndexDB) deleteFullText(whereClause string, args ...interface{}) (int, error) {
	if db.fullTextOff() {
		return 0, nil
	}
	res, err := db.Exec("DELETE FROM index_fts WHERE "+whereClause, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// PurgeFullTextOrphans removes full-text rows under pathPrefix that no longer have an
// index_items row (needed after stale sweeps delete rows by rowid). Returns rows removed.
func (db *IndexDB) PurgeFullTextOrphans(source, pathPrefix string) (int, error) {
	nextPrefix := getNextPathPrefix(pathPrefix)
	return db.deleteFullText(`source = ? AND path >= ? AND path < ?
		AND path NOT IN (SELECT path FROM index_items WHERE source = ? AND path >= ? AND path < ?)`,
		source, pathPrefix, nextPrefix, source, pathPrefix, nextPrefix)
}
