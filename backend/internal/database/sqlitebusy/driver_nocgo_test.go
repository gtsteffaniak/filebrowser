//go:build !cgosql
// +build !cgosql

package sqlitebusy

import (
	_ "modernc.org/sqlite"
)

const testDriver = "sqlite"
