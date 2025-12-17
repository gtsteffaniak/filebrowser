//go:build cgo

package sql

import (
	_ "github.com/mattn/go-sqlite3" // CGO-based SQLite driver
)

// sqliteDriverName is the name used when opening the database connection.
// mattn/go-sqlite3 registers itself as "sqlite3".
const sqliteDriverName = "sqlite3"

