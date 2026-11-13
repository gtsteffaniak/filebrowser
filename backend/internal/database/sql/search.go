package sql

import (
	"database/sql"
	"fmt"
	"strings"
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

// appendNameGlobSQL adds AND (name GLOB ? OR ...) or AND (name GLOB ? AND ...) when joinAnd is true.
func appendNameGlobSQL(query string, args []interface{}, patterns []string, joinAnd, caseExact bool) (string, []interface{}) {
	if len(patterns) == 0 {
		return query, args
	}
	nameExpr := "name"
	if !caseExact {
		nameExpr = "lower(name)"
	}
	parts := make([]string, len(patterns))
	for i := range patterns {
		parts[i] = nameExpr + " GLOB ?"
		args = append(args, patterns[i])
	}
	join := " OR "
	if joinAnd {
		join = " AND "
	}
	return query + " AND (" + strings.Join(parts, join) + ")", args
}

// orderByPathDepth returns the ORDER BY clause matching the search cursor contract:
// largest searches rank by size, everything else by (length(path), path).
func orderByPathDepth(largest bool) string {
	if largest {
		return " ORDER BY size DESC"
	}
	return " ORDER BY length(path), path"
}

// SearchItems queries the database for items matching the search criteria for a single source.
// When nameGlobPatterns is non-empty (and largest is false), rows are restricted with SQLite name GLOB ... OR ....
// Returns rows that can be iterated to scan search results.
func (db *IndexDB) SearchItems(source string, scope string, largest bool, nameGlobPatterns []string, nameGlobPatternsAnd, caseExact bool, cursorPath string) (*sql.Rows, error) {
	query := `
		SELECT path, name, size, created_time, mod_time, type, is_dir, has_preview
		FROM index_items
		WHERE source = ?
	`
	args := []interface{}{source}

	// Apply scope filter
	if scope != "" {
		// Use GLOB for prefix matching which is supported by SQLite and efficient
		query += " AND path GLOB ?"
		args = append(args, scope+"*")
	}

	if !largest && len(nameGlobPatterns) > 0 {
		query, args = appendNameGlobSQL(query, args, nameGlobPatterns, nameGlobPatternsAnd, caseExact)
	}

	// cursorPath pages past everything ordered at or before it; compare with the same
	// SQL length unit, so callers never have to compute a length themselves.
	if cursorPath != "" {
		query += " AND (length(path) > length(?) OR (length(path) = length(?) AND path > ?))"
		args = append(args, cursorPath, cursorPath, cursorPath)
	}
	// Without this ORDER BY the row limit would truncate an arbitrary subset, and the
	// caller's (length(path), path) cursor would skip whatever was never read.
	query += orderByPathDepth(largest)

	return db.Query(query, args...)
}

// SearchItemsMultiSource queries the database for items matching the search criteria across multiple sources.
// When nameGlobPatterns is non-empty (and largest is false), restricts rows with SQLite name GLOB ... OR ....
// Returns rows that can be iterated to scan search results.
func (db *IndexDB) SearchItemsMultiSource(sources []string, sourceScopes map[string]string, largest bool, nameGlobPatterns []string, nameGlobPatternsAnd, caseExact bool, cursorPath string) (*sql.Rows, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("at least one source is required")
	}

	query := `
		SELECT source, path, name, size, created_time, mod_time, type, is_dir, has_preview
		FROM index_items
	`
	args := []interface{}{}
	whereClauses := []string{}

	// Filter by sources using IN clause
	if len(sources) == 1 {
		whereClauses = append(whereClauses, "source = ?")
		args = append(args, sources[0])
	} else {
		placeholders := make([]string, len(sources))
		for i, source := range sources {
			placeholders[i] = "?"
			args = append(args, source)
		}
		whereClauses = append(whereClauses, "source IN ("+strings.Join(placeholders, ",")+")")
	}

	// Apply scope filters - need OR conditions for each source+scope combination
	// Build conditions for each source: if it has a scope, filter by it; otherwise include all paths for that source
	scopeConditions := []string{}
	for _, source := range sources {
		scope, hasScope := sourceScopes[source]
		if hasScope && scope != "" {
			// Source has a scope - filter by it
			scopeConditions = append(scopeConditions, "(source = ? AND path GLOB ?)")
			args = append(args, source, scope+"*")
		} else {
			// Source has no scope or empty scope - include all paths for this source
			// The source IN clause already filters by source, so we just need to ensure
			// this source is included in the OR condition (which it already is via source IN)
			// We add a simple source check to make the OR condition work correctly
			scopeConditions = append(scopeConditions, "source = ?")
			args = append(args, source)
		}
	}
	if len(scopeConditions) > 0 {
		whereClauses = append(whereClauses, "("+strings.Join(scopeConditions, " OR ")+")")
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

	if !largest && len(nameGlobPatterns) > 0 {
		query, args = appendNameGlobSQL(query, args, nameGlobPatterns, nameGlobPatternsAnd, caseExact)
	}

	// cursorPath pages past everything ordered at or before it; compare with the same
	// SQL length unit, so callers never have to compute a length themselves.
	if cursorPath != "" {
		query += " AND (length(path) > length(?) OR (length(path) = length(?) AND path > ?))"
		args = append(args, cursorPath, cursorPath, cursorPath)
	}
	// Without this ORDER BY the row limit would truncate an arbitrary subset, and the
	// caller's (length(path), path) cursor would skip whatever was never read.
	query += orderByPathDepth(largest)

	return db.Query(query, args...)
}
