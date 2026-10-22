//go:build cgosql && !sqlite_fts5
// +build cgosql,!sqlite_fts5

package sql

// The cgo SQLite driver (mattn/go-sqlite3) only enables FTS5 when built with the
// sqlite_fts5 build tag. The trigram search index requires FTS5, so this build
// combination is rejected at compile time. Build with:
//
//	go build -tags "cgosql sqlite_fts5"
//
// The reference to an undefined symbol below is intentional: it makes the cgo build
// fail loudly instead of silently falling back to unindexed search.
var _ = cgosql_requires_the_sqlite_fts5_build_tag
