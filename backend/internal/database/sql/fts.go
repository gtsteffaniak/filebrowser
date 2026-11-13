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

// SearchFullText returns index paths for source whose body matches every term
// (prefix matching, AND-combined). scope is a directory index path ending in "/".
func (db *IndexDB) SearchFullText(source, scope string, terms []string, limit int) ([]string, error) {
	expr := fullTextMatchExpr(terms)
	if db.fullTextDisabled || expr == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 100
	}
	// index_items is not joined here: doing so made the planner scan index_fts
	// instead of using the FTS index; scope filtering is a cheap GLOB on the FTS rows.
	nameGlob := sqliteGlobEscape(scope) + "*"
	rows, err := db.Query(`
		SELECT path FROM index_fts
		WHERE body MATCH ? AND source = ? AND path GLOB ?
		LIMIT ?`, expr, source, nameGlob, limit)
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
func fullTextMatchExpr(terms []string) string {
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		parts = append(parts, `"`+strings.ReplaceAll(t, `"`, `""`)+`"*`)
	}
	return strings.Join(parts, " AND ")
}

// sqliteGlobEscape escapes GLOB metacharacters so a literal path prefix stays literal.
func sqliteGlobEscape(s string) string {
	return strings.NewReplacer("*", "[*]", "?", "[?]", "[", "[[]").Replace(s)
}

// upsertFullText replaces the content row for one freshly upserted index_items row.
// The old row is dropped first so an upsert without a body never leaves stale text searchable.
func upsertFullText(tx *sql.Tx, stmt *sql.Stmt, res sql.Result, source string, info *iteminfo.FileInfo) error {
	empty := info.FullTextBody == nil || *info.FullTextBody == ""
	rowid, err := res.LastInsertId()
	if err != nil || rowid == 0 {
		if err := tx.QueryRow(`SELECT rowid FROM index_items WHERE source = ? AND path = ?`, source, info.Path).Scan(&rowid); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM index_fts WHERE rowid = ?`, rowid); err != nil {
		return err
	}
	if empty {
		return nil
	}
	_, err = stmt.Exec(rowid, source, info.Path, *info.FullTextBody)
	return err
}

// deleteFullText removes full-text rows matching whereClause (same args as the index_items delete).
// Newly re-indexed items get their row back on the next scan.
func (db *IndexDB) deleteFullText(whereClause string, args ...interface{}) error {
	db.fullTextMu.Lock()
	disabled := db.fullTextDisabled
	db.fullTextMu.Unlock()
	if disabled {
		return nil
	}
	_, err := db.Exec("DELETE FROM index_fts WHERE "+whereClause, args...)
	return err
}

// purgeFullTextOrphans removes full-text rows under pathPrefix that no longer have an index_items row.
func (db *IndexDB) purgeFullTextOrphans(source, pathPrefix string) error {
	nextPrefix := getNextPathPrefix(pathPrefix)
	return db.deleteFullText(`source = ? AND path >= ? AND path < ?
		AND path NOT IN (SELECT path FROM index_items WHERE source = ? AND path >= ? AND path < ?)`,
		source, pathPrefix, nextPrefix, source, pathPrefix, nextPrefix)
}
