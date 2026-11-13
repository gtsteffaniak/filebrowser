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
// Results are ordered by (length(path), path); startingAfter pages past that key.
// StartingAfter selects the resume predicate: rows ordered after startingAfter for
// sources at or past the cursor source, and rows from startingAfter itself for sources
// ordered before it (multi-source cursor (length(path), path, source)).
const (
	StartingAfter = iota
	StartingAt
)

func (db *IndexDB) SearchFullText(source, scope string, terms []string, matchAll bool, limit int, startingAfter string) ([]string, error) {
	return db.SearchFullTextFrom(source, scope, terms, matchAll, limit, startingAfter, StartingAfter)
}

// searchFullTextFrom pages content hits starting strictly after, or at, startingAfter.
func (db *IndexDB) SearchFullTextFrom(source, scope string, terms []string, matchAll bool, limit int, startingAfter string, mode int) ([]string, error) {
	expr := fullTextMatchExpr(terms, matchAll)
	if db.fullTextOff() || expr == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	// index_items is not joined here: doing so made the planner scan index_fts
	// instead of using the FTS index; scope filtering is a cheap GLOB on the FTS rows.
	nameGlob := sqliteGlobEscape(scope) + "*"
	// Order by (length(path), path) before LIMIT so a limited page keeps the
	// shallowest, then lexicographically first, matches. The cursor is the last path
	// returned; its length is computed by SQLite so both sides share one length unit.
	rows, err := db.Query(`
		SELECT path FROM index_fts
		WHERE body MATCH ? AND source = ? AND path GLOB ?
		AND (length(path) > length(?) OR (length(path) = length(?) AND (path > ? OR (? = 1 AND path = ?))))
		ORDER BY length(path), path
		LIMIT ?`, expr, source, nameGlob, startingAfter, startingAfter, startingAfter, mode, startingAfter, limit)
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

// upsertFullText writes the content row for one freshly upserted index_items row.
// rowid comes from an indexed (source, path) lookup of index_items, never from
// LastInsertId: after ON CONFLICT DO UPDATE that reports an earlier row.
// source/path are stored alongside for FTS queries that cannot join index_items.
// An empty body writes nothing; the old FTS row was already deleted by the caller.
func upsertFullText(stmt *sql.Stmt, rowid int64, source string, info *iteminfo.FileInfo) error {
	if info.FullTextBody == nil {
		return nil
	}
	_, err := stmt.Exec(rowid, source, info.Path, *info.FullTextBody)
	return err
}

// deleteItemRowsTx deletes matching index_items and index_fts rows in one transaction so a
// concurrent insert cannot interleave between the two deletes.
func (db *IndexDB) deleteItemRowsTx(whereClause string, args ...interface{}) error {
	committed := false
	tx, err := db.BeginTransaction()
	if err != nil {
		// BeginTransaction locks mu before Begin and does not unlock on error.
		db.mu.Unlock()
		return err
	}
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
