//go:build cgosql
// +build cgosql

package sqlitebusy

import (
	"errors"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// driverResultCode extracts the SQLite primary result code from err using the
// CGO (mattn/go-sqlite3) driver's error type.
func driverResultCode(err error) (int, bool) {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return int(sqliteErr.Code), true
	}
	return 0, false
}
