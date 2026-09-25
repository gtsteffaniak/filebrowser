package sqlitebusy

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// DefaultBusyTimeoutMs is how long SQLite waits on SQLITE_BUSY before failing an operation.
const DefaultBusyTimeoutMs = 5000

// ErrBusy is returned when an operation could not complete because the database was busy or locked.
var ErrBusy = errors.New("sqlite database is busy or locked")

// ApplyBusyTimeout sets PRAGMA busy_timeout on db.
func ApplyBusyTimeout(db *sql.DB) error {
	if db == nil {
		return errors.New("nil database")
	}
	_, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d", DefaultBusyTimeoutMs))
	return err
}

// IsBusyOrLocked reports SQLITE_BUSY / database is locked style errors from modernc or mattn drivers.
func IsBusyOrLocked(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "database is locked") ||
		strings.Contains(errStr, "SQLITE_BUSY") ||
		strings.Contains(errStr, "(5)") ||
		isTransactionError(err)
}

func isTransactionError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "cannot start a transaction within a transaction") ||
		strings.Contains(errStr, "cannot commit") ||
		strings.Contains(errStr, "(1)")
}

// Wrap returns err wrapped with ErrBusy when the underlying error is busy/locked.
func Wrap(err error) error {
	if err == nil || !IsBusyOrLocked(err) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrBusy, err)
}
