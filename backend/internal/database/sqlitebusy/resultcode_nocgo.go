//go:build !cgosql
// +build !cgosql

package sqlitebusy

import "errors"

// codeError matches the result-code-bearing error type exposed by the pure-Go
// (modernc.org/sqlite) driver (*sqlite.Error). The code may be an extended
// result code; the primary code is isolated by isBusyResultCode.
type codeError interface {
	error
	Code() int
}

func driverResultCode(err error) (int, bool) {
	var sqliteErr codeError
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code(), true
	}
	return 0, false
}
