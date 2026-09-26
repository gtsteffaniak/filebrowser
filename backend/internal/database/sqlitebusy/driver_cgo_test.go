//go:build cgosql
// +build cgosql

package sqlitebusy

import (
	"errors"

	sqlite3 "github.com/mattn/go-sqlite3"
)

const testDriver = "sqlite3"

// driverErrorCode returns the driver's extended result code for err, so tests
// can assert which busy/locked variant was produced. mattn reports the
// extended code in Error.ExtendedCode.
func driverErrorCode(err error) int {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return int(sqliteErr.ExtendedCode)
	}
	return -1
}
