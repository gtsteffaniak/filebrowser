package sqlitebusy

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestIsBusyResultCode(t *testing.T) {
	// Primary and extended SQLITE_BUSY / SQLITE_LOCKED result codes.
	// e.g. SQLITE_BUSY_RECOVERY (261), SQLITE_BUSY_SNAPSHOT (517),
	// SQLITE_LOCKED_SHAREDCACHE (262), SQLITE_LOCKED_VTAB (518).
	for _, code := range []int{5, 6, 261, 517, 773, 262, 518} {
		if !isBusyResultCode(code) {
			t.Fatalf("expected result code %d to be busy/locked", code)
		}
	}
	// Extended codes of other primaries must not be classified as busy.
	for _, code := range []int{0, 1, 8, 10, 19, 266, 2067, 264, 1544} {
		if isBusyResultCode(code) {
			t.Fatalf("expected result code %d not to be busy/locked", code)
		}
	}
}

// TestIsBusyOrLockedExtendedCodes verifies extended busy/locked result codes
// are classified through the driver error path and wrapped with ErrBusy.
func TestIsBusyOrLockedExtendedCodes(t *testing.T) {
	for _, code := range []int{517, 261, 773, 262, 518} {
		if !IsBusyOrLocked(newCodeError(code)) {
			t.Fatalf("expected extended code %d to be busy/locked", code)
		}
		if err := Wrap(newCodeError(code)); !errors.Is(err, ErrBusy) {
			t.Fatalf("expected ErrBusy for extended code %d, got %v", code, err)
		}
	}
	for _, code := range []int{1, 266, 2067, 264, 1544} {
		if IsBusyOrLocked(newCodeError(code)) {
			t.Fatalf("expected code %d not to be busy/locked", code)
		}
		if err := Wrap(newCodeError(code)); errors.Is(err, ErrBusy) {
			t.Fatalf("expected code %d to pass through unwrapped, got %v", code, err)
		}
	}
}

func TestIsBusyOrLocked(t *testing.T) {
	if IsBusyOrLocked(nil) {
		t.Fatal("expected nil not to be busy")
	}
	if IsBusyOrLocked(newGenericError(t)) {
		t.Fatal("expected non-busy SQLITE_ERROR not to be classified as busy")
	}
	if !IsBusyOrLocked(newBusyError(t)) {
		t.Fatal("expected SQLITE_BUSY to be classified as busy")
	}
}

func TestWrap(t *testing.T) {
	wrapped := Wrap(newBusyError(t))
	if !errors.Is(wrapped, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", wrapped)
	}

	generic := newGenericError(t)
	if err := Wrap(generic); errors.Is(err, ErrBusy) {
		t.Fatalf("expected generic error to pass through unwrapped, got %v", err)
	}
	if err := Wrap(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestWithBusyTimeoutAppliesToEveryConnection verifies the DSN parameter is
// applied at connection-open time, so a second pooled connection does not
// bypass the timeout like a one-off PRAGMA would.
func TestWithBusyTimeoutAppliesToEveryConnection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pool.db")
	db, err := sql.Open(testDriver, WithBusyTimeout("file:"+dbPath))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)

	ctx := context.Background()
	conn1, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn1: %v", err)
	}
	defer conn1.Close()
	conn2, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("conn2: %v", err)
	}
	defer conn2.Close()

	for i, conn := range []*sql.Conn{conn1, conn2} {
		var timeout int
		if err := conn.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&timeout); err != nil {
			t.Fatalf("conn %d: read busy_timeout: %v", i+1, err)
		}
		if timeout != DefaultBusyTimeoutMs {
			t.Fatalf("conn %d: busy_timeout = %d, want %d", i+1, timeout, DefaultBusyTimeoutMs)
		}
	}
}

// newGenericError returns a real SQLITE_ERROR (result code 1) from the driver.
func newGenericError(t *testing.T) error {
	t.Helper()
	db, err := sql.Open(testDriver, filepath.Join(t.TempDir(), "generic.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	_, err = db.Exec("SELECT * FROM table_that_does_not_exist")
	if err == nil {
		t.Fatal("expected error from invalid query")
	}
	return err
}

// newBusyError returns a real SQLITE_BUSY (result code 5) from the driver by
// holding a write lock on one connection and writing from a second with a zero
// busy timeout.
func newBusyError(t *testing.T) error {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "busy.db")

	holder, err := sql.Open(testDriver, WithBusyTimeout("file:"+dbPath))
	if err != nil {
		t.Fatalf("open holder: %v", err)
	}
	defer holder.Close()
	holder.SetMaxOpenConns(1)

	if _, err = holder.Exec("CREATE TABLE t (id INTEGER)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	tx, err := holder.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.Exec("INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("holder insert: %v", err)
	}

	contender, err := sql.Open(testDriver, "file:"+dbPath+"?_busy_timeout=0")
	if err != nil {
		t.Fatalf("open contender: %v", err)
	}
	defer contender.Close()

	_, err = contender.Exec("INSERT INTO t VALUES (2)")
	if err == nil {
		t.Fatal("expected SQLITE_BUSY from contended write")
	}
	return err
}
