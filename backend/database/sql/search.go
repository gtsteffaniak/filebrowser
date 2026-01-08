package sql

import (
	"database/sql"
	"fmt"
	"strings"
)

// SearchItem represents a single search result row from the database
type SearchItem struct {
	Source     string
	Path       string
	Name       string
	Size       int64
	ModTime    int64
	Type       string
	IsDir      bool
	HasPreview bool
}

// SearchItems queries the database for items matching the search criteria for a single source.
// Returns rows that can be iterated to scan search results.
func (db *IndexDB) SearchItems(source string, scope string, largest bool) (*sql.Rows, error) {
	query := `
		SELECT path, name, size, mod_time, type, is_dir, has_preview 
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

	if largest {
		query += " ORDER BY size DESC"
	}

	return db.Query(query, args...)
}

// SearchItemsMultiSource queries the database for items matching the search criteria across multiple sources.
// Returns rows that can be iterated to scan search results.
func (db *IndexDB) SearchItemsMultiSource(sources []string, sourceScopes map[string]string, largest bool) (*sql.Rows, error) {
	if len(sources) == 0 {
		return nil, fmt.Errorf("at least one source is required")
	}

	query := `
		SELECT source, path, name, size, mod_time, type, is_dir, has_preview 
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

	if largest {
		query += " ORDER BY size DESC"
	}

	return db.Query(query, args...)
}
