//go:build !cgosql
// +build !cgosql

package sqlitebusy

import (
	"errors"

	sqlite "modernc.org/sqlite"
)

// driverResultCode extracts the SQLite primary result code from err using the
// pure-Go (modernc.org/sqlite) driver's error type.
func driverResultCode(err error) (int, bool) {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code(), true
	}
	return 0, false
}
