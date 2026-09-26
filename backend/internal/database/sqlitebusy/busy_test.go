package sqlitebusy

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

// sqliteBusySnapshot is the extended result code returned when a WAL read
// transaction tries to upgrade to a write on a stale snapshot.
const sqliteBusySnapshot = 517

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
	if !IsBusyOrLocked(newBusySnapshotError(t)) {
		t.Fatal("expected SQLITE_BUSY_SNAPSHOT to be classified as busy")
	}
}

func TestWrap(t *testing.T) {
	for name, makeErr := range map[string]func(*testing.T) error{
		"SQLITE_BUSY":          newBusyError,
		"SQLITE_BUSY_SNAPSHOT": newBusySnapshotError,
	} {
		t.Run(name, func(t *testing.T) {
			wrapped := Wrap(makeErr(t))
			if !errors.Is(wrapped, ErrBusy) {
				t.Fatalf("expected ErrBusy, got %v", wrapped)
			}
		})
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

// newBusySnapshotError returns a real extended-busy error (SQLITE_BUSY_SNAPSHOT)
// from the driver. In WAL mode, a transaction that has already taken a read
// snapshot cannot upgrade to a write after another connection committed —
// SQLite returns SQLITE_BUSY_SNAPSHOT instead of waiting on busy_timeout.
func newBusySnapshotError(t *testing.T) error {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "snapshot.db")
	dsn := "file:" + dbPath + "?_journal_mode=WAL&_busy_timeout=0"

	db, err := sql.Open(testDriver, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(2)

	if _, err = db.Exec("CREATE TABLE t (id INTEGER); INSERT INTO t VALUES (1)"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	ctx := context.Background()
	reader, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("reader conn: %v", err)
	}
	defer reader.Close()

	tx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Establish the read snapshot.
	var count int
	if err = tx.QueryRow("SELECT COUNT(*) FROM t").Scan(&count); err != nil {
		t.Fatalf("snapshot read: %v", err)
	}

	// Commit a write from a different connection, invalidating the snapshot.
	writer, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("writer conn: %v", err)
	}
	defer writer.Close()
	if _, err = writer.ExecContext(ctx, "INSERT INTO t VALUES (2)"); err != nil {
		t.Fatalf("writer insert: %v", err)
	}

	// Upgrading the stale snapshot to a write must fail with SQLITE_BUSY_SNAPSHOT.
	_, err = tx.Exec("INSERT INTO t VALUES (3)")
	if err == nil {
		t.Fatal("expected SQLITE_BUSY_SNAPSHOT from write on stale snapshot")
	}
	if code := driverErrorCode(err); code != sqliteBusySnapshot {
		t.Fatalf("expected extended code %d (SQLITE_BUSY_SNAPSHOT), got %d", sqliteBusySnapshot, code)
	}
	return err
}
