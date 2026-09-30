//go:build !cgosql
// +build !cgosql

package sqlitebusy

import (
	"errors"

	sqlite "modernc.org/sqlite"
)

const testDriver = "sqlite"

// driverErrorCode returns the driver's extended result code for err, so tests
// can assert which busy/locked variant was produced. modernc reports the
// (possibly extended) code via Error.Code.
func driverErrorCode(err error) int {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code()
	}
	return -1
}
