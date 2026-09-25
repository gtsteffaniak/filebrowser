//go:build cgosql
// +build cgosql

package sqlitebusy

import (
	"errors"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// isBusyOrLocked reports whether err is a SQLITE_BUSY or SQLITE_LOCKED error
// from the CGO (mattn/go-sqlite3) driver. Error.Code is already the primary
// result code; extended codes are reported separately in Error.ExtendedCode.
func isBusyOrLocked(err error) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked
}
