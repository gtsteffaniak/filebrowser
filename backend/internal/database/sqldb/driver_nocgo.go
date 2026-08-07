//go:build !cgosql
// +build !cgosql

package sqldb

import (
	_ "modernc.org/sqlite"
)

const SqliteDriver = "sqlite"
