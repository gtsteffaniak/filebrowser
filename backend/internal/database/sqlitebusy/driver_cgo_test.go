//go:build cgosql
// +build cgosql

package sqlitebusy

import (
	_ "github.com/mattn/go-sqlite3"
)

const testDriver = "sqlite3"
