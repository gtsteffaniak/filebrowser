//go:build cgosql
// +build cgosql

package sqldb

import (
	_ "github.com/mattn/go-sqlite3"
)

const SqliteDriver = "sqlite3"
