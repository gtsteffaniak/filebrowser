//go:build cgosql
// +build cgosql

package sqlitebusy

import (
	sqlite3 "github.com/mattn/go-sqlite3"
)

const testDriver = "sqlite3"

// newCodeError builds a driver error carrying the given (possibly extended)
// result code. The mattn driver splits codes into the primary Code and the
// ExtendedCode field.
func newCodeError(code int) error {
	return sqlite3.Error{
		Code:         sqlite3.ErrNo(code & 0xff),
		ExtendedCode: sqlite3.ErrNoExtended(code),
	}
}
