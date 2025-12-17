//go:build !cgo

package sql

import (
	_ "modernc.org/sqlite" // Pure Go SQLite driver (no CGO required)
)

// sqliteDriverName is the name used when opening the database connection.
// modernc.org/sqlite registers itself as "sqlite".
const sqliteDriverName = "sqlite"
