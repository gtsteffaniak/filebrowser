//go:build cgosql
// +build cgosql

// The cgo driver requires the sqlite_fts5 build tag so the FTS5 trigram search index
// is available: build with -tags "cgosql sqlite_fts5" (see driver_cgo_fts5_guard.go).
package sql

import (
	_ "github.com/mattn/go-sqlite3"
)

const SqliteDriver = "sqlite3"
