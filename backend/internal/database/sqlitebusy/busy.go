package sqlitebusy

import (
	"errors"
	"fmt"
	"strings"
)

// DefaultBusyTimeoutMs is how long SQLite waits on SQLITE_BUSY before failing an operation.
const DefaultBusyTimeoutMs = 5000

// SQLite primary result codes we treat as retryable.
const (
	sqliteBusyCode   = 5 // SQLITE_BUSY
	sqliteLockedCode = 6 // SQLITE_LOCKED
)

// ErrBusy is returned when an operation could not complete because the database was busy or locked.
var ErrBusy = errors.New("sqlite database is busy or locked")

// WithBusyTimeout appends the busy_timeout connection parameter to dsn so that
// every connection in the pool honors the timeout. Both supported drivers
// (mattn/go-sqlite3 and modernc.org/sqlite) apply the parameter at
// connection-open time, unlike a one-off PRAGMA which only configures the
// single connection that database/sql happens to pick.
func WithBusyTimeout(dsn string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return fmt.Sprintf("%s%s_busy_timeout=%d", dsn, separator, DefaultBusyTimeoutMs)
}

// IsBusyOrLocked reports whether err is SQLITE_BUSY or SQLITE_LOCKED from the
// configured SQLite driver, classified by result code rather than error text.
func IsBusyOrLocked(err error) bool {
	if err == nil {
		return false
	}
	code, ok := driverResultCode(err)
	return ok && isBusyResultCode(code)
}

func isBusyResultCode(code int) bool {
	// Only the lowest byte is the primary result code; higher bits carry
	// extended detail (e.g. SQLITE_BUSY_SNAPSHOT, SQLITE_LOCKED_SHAREDCACHE).
	switch code & 0xff {
	case sqliteBusyCode, sqliteLockedCode:
		return true
	default:
		return false
	}
}

// Wrap returns err wrapped with ErrBusy when the underlying error is busy/locked.
// The original error is retained so callers can still inspect the driver result code.
func Wrap(err error) error {
	if err == nil || !IsBusyOrLocked(err) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrBusy, err)
}
