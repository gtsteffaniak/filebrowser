//go:build !cgosql
// +build !cgosql

package sqlitebusy

import (
	"errors"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// isBusyOrLocked reports whether err is a SQLITE_BUSY or SQLITE_LOCKED error
// from the pure-Go (modernc.org/sqlite) driver. The driver enables extended
// result codes, so Error.Code() may carry extra detail in the high bits (e.g.
// SQLITE_BUSY_SNAPSHOT); only the lowest byte holds the primary code.
func isBusyOrLocked(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	switch sqliteErr.Code() & 0xff {
	case sqlite3.SQLITE_BUSY, sqlite3.SQLITE_LOCKED:
		return true
	default:
		return false
	}
}
