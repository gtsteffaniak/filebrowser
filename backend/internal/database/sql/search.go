package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// SearchItem represents a single search result row from the database
type SearchItem struct {
	Source      string
	Path        string
	Name        string
	Size        int64
	CreatedTime int64
	ModTime     int64
	Type        string
	IsDir       bool
	HasPreview  bool
}

// appendGlob adds "AND (col GLOB ? [OR/AND] ...)" when patterns is non-empty.
func appendGlob(query string, args []interface{}, column string, patterns []string, joinAnd bool) (string, []interface{}) {
	if len(patterns) == 0 {
		return query, args
	}
	parts := make([]string, len(patterns))
	for i, p := range patterns {
		parts[i] = column + " GLOB ?"
		args = append(args, p)
	}
	join := " OR "
	if joinAnd {
		join = " AND "
	}
	return query + " AND (" + strings.Join(parts, join) + ")", args
}

// appendLike adds "AND (col LIKE ? [OR/AND] ...)" when patterns is non-empty.
func appendLike(query string, args []interface{}, column string, patterns []string, joinAnd bool) (string, []interface{}) {
	if len(patterns) == 0 {
		return query, args
	}
	parts := make([]string, len(patterns))
	for i, p := range patterns {
		parts[i] = column + " LIKE ?"
		args = append(args, p)
	}
	join := " OR "
	if joinAnd {
		join = " AND "
	}
	return query + " AND (" + strings.Join(parts, join) + ")", args
}

// useFTSPath reports whether the trigram index can serve this name filter.
func useFTSPath(db *IndexDB, filter *iteminfo.NameFilter) bool {
	return filter != nil && filter.UseFTS && db.FTSReady()
}

// appendNamePrefilter narrows rows using the FTS trigram index (a superset of the
// exact predicate). It is only valid in the FTS query path.
func appendNamePrefilter(query string, args []interface{}, filter *iteminfo.NameFilter) (string, []interface{}) {
	if filter == nil || len(filter.FTS) == 0 {
		return query, args
	}
	if filter.PrefilterLike {
		return appendLike(query, args, "f.name", filter.FTS, filter.JoinAnd)
	}
	return appendGlob(query, args, "f.name", filter.FTS, filter.JoinAnd)
}

// appendNameExact adds the exact name predicate applied to the base table. alias is
// "" for the single-table query or "i." in the FTS join.
func appendNameExact(query string, args []interface{}, filter *iteminfo.NameFilter, alias string) (string, []interface{}) {
	if filter == nil || len(filter.Exact) == 0 {
		return query, args
	}
	column := alias + "name"
	if filter.ExactLower {
		column = "lower(" + alias + "name)"
	}
	return appendGlob(query, args, column, filter.Exact, filter.JoinAnd)
}

// SearchItems queries the database for items matching the search criteria for a single source.
// When the filter is usable and the FTS index is ready, rows are narrowed with the trigram
// index and then filtered by the exact name predicate; otherwise the base table is scanned.
func (db *IndexDB) SearchItems(ctx context.Context, source, scope string, largest bool, filter *iteminfo.NameFilter) (*sql.Rows, error) {
	if largest {
		query := `
			SELECT path, name, size, created_time, mod_time, type, is_dir, has_preview
			FROM index_items
			WHERE source = ?
		`
		args := []interface{}{source}
		if scope != "" {
			query += " AND path GLOB ?"
			args = append(args, iteminfo.EscapeGlobLiteral(scope)+"*")
		}
		query += " ORDER BY size DESC"
		return db.QueryContext(ctx, query, args...)
	}

	if useFTSPath(db, filter) {
		query := `
			SELECT i.path, i.name, i.size, i.created_time, i.mod_time, i.type, i.is_dir, i.has_preview
			FROM index_items_fts f CROSS JOIN index_items i ON i.rowid = f.rowid
			WHERE i.source = ?
		`
		args := []interface{}{source}
		if scope != "" {
			query += " AND i.path GLOB ?"
			args = append(args, iteminfo.EscapeGlobLiteral(scope)+"*")
		}
		// Narrow by source+scope inside the global FTS index (superset; exact filters below).
		query += " AND f.spath LIKE ?"
		args = append(args, source+":"+scope+"%")
		query, args = appendNamePrefilter(query, args, filter)
		query, args = appendNameExact(query, args, filter, "i.")
		return db.QueryContext(ctx, query, args...)
	}

	query := `
		SELECT path, name, size, created_time, mod_time, type, is_dir, has_preview
		FROM index_items
		WHERE source = ?
	`
	args := []interface{}{source}
	if scope != "" {
		query += " AND path GLOB ?"
		args = append(args, iteminfo.EscapeGlobLiteral(scope)+"*")
	}
	query, args = appendNameExact(query, args, filter, "")
	return db.QueryContext(ctx, query, args...)
}

// SearchItemsMultiSource queries the database for items matching the search criteria across multiple sources.
func (db *IndexDB) SearchItemsMultiSource(ctx context.Context, sources []string, sourceScopes map[string]string, largest bool, filter *iteminfo.NameFilter) (*sql.Rows, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("at least one source is required")
	}

	args := []interface{}{}
	placeholders := make([]string, len(sources))
	for i, source := range sources {
		placeholders[i] = "?"
		args = append(args, source)
	}
	sourceClause := "source = ?"
	if len(sources) > 1 {
		sourceClause = "source IN (" + strings.Join(placeholders, ",") + ")"
	}

	// Per-source scope conditions (on the base table).
	scopeConditions := []string{}
	scopeArgs := []interface{}{}
	spathConditions := []string{}
	spathArgs := []interface{}{}
	for _, source := range sources {
		scope, hasScope := sourceScopes[source]
		if hasScope && scope != "" {
			scopeConditions = append(scopeConditions, "(source = ? AND path GLOB ?)")
			scopeArgs = append(scopeArgs, source, iteminfo.EscapeGlobLiteral(scope)+"*")
			spathConditions = append(spathConditions, "f.spath LIKE ?")
			spathArgs = append(spathArgs, source+":"+scope+"%")
		} else {
			scopeConditions = append(scopeConditions, "source = ?")
			scopeArgs = append(scopeArgs, source)
		}
	}

	if largest {
		query := `
			SELECT source, path, name, size, created_time, mod_time, type, is_dir, has_preview
			FROM index_items
			WHERE ` + sourceClause
		query += " AND (" + strings.Join(scopeConditions, " OR ") + ")"
		query += " ORDER BY size DESC"
		allArgs := append(args, scopeArgs...)
		return db.QueryContext(ctx, query, allArgs...)
	}

	if useFTSPath(db, filter) {
		query := `
			SELECT i.source, i.path, i.name, i.size, i.created_time, i.mod_time, i.type, i.is_dir, i.has_preview
			FROM index_items_fts f CROSS JOIN index_items i ON i.rowid = f.rowid
			WHERE i.` + sourceClause
		query += " AND (" + strings.Join(scopeConditions, " OR ") + ")"
		allArgs := append(args, scopeArgs...)
		if len(spathConditions) > 0 {
			query += " AND (" + strings.Join(spathConditions, " OR ") + ")"
			allArgs = append(allArgs, spathArgs...)
		}
		query, allArgs = appendNamePrefilter(query, allArgs, filter)
		query, allArgs = appendNameExact(query, allArgs, filter, "i.")
		return db.QueryContext(ctx, query, allArgs...)
	}

	query := `
		SELECT source, path, name, size, created_time, mod_time, type, is_dir, has_preview
		FROM index_items
		WHERE ` + sourceClause
	query += " AND (" + strings.Join(scopeConditions, " OR ") + ")"
	allArgs := append(args, scopeArgs...)
	query, allArgs = appendNameExact(query, allArgs, filter, "")
	return db.QueryContext(ctx, query, allArgs...)
}
