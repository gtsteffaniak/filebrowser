//go:build !cgosql
// +build !cgosql

package sql

import (
	_ "modernc.org/sqlite"
)

const SqliteDriver = "sqlite"
