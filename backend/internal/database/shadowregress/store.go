// Package shadowregress stores significant go-media vs ffmpeg regressions found
// by dev-mode shadow runs, so admins can review them in the settings UI.
//
// The database holds one row per file. A file is identified by a hash of its
// path, so repeated regressions for the same file update that row (latest
// detail, plus an occurrence count) instead of adding entries.
//
// The database is a separate SQLite file in the cache directory's sql/ folder,
// created the same way as the index database. It is opened lazily, so nothing
// is created until the first regression is recorded or the viewer is opened.
package shadowregress

import (
	"errors"
	"fmt"
	"sync"
	"time"

	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
)

// MaxEntries is the most files kept. The least recently seen rows are pruned on insert.
const MaxEntries = 1000

const schema = `
CREATE TABLE IF NOT EXISTS shadow_regressions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	file_key TEXT NOT NULL UNIQUE,
	first_seen INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	operation TEXT NOT NULL,
	label TEXT NOT NULL,
	detail TEXT NOT NULL,
	format TEXT NOT NULL,
	size_bytes INTEGER NOT NULL,
	codecs TEXT NOT NULL DEFAULT '',
	occurrences INTEGER NOT NULL DEFAULT 1
);`

// incompatibleSchema holds files go-media does not support that ffmpeg handled.
// Unlike shadow_regressions, it stores the file path so the list can name the files.
const incompatibleSchema = `
CREATE TABLE IF NOT EXISTS shadow_incompatible (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	file_key TEXT NOT NULL UNIQUE,
	first_seen INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	operation TEXT NOT NULL,
	path TEXT NOT NULL,
	format TEXT NOT NULL,
	size_bytes INTEGER NOT NULL,
	occurrences INTEGER NOT NULL DEFAULT 1
);`

// IncompatibleEntry is one file go-media does not support that ffmpeg handled.
type IncompatibleEntry struct {
	FileKey     string    `json:"-"`
	FirstSeen   time.Time `json:"firstSeen"`
	CreatedAt   time.Time `json:"createdAt"`
	Occurrences int64     `json:"occurrences"`
	Operation   string    `json:"operation"`
	Path        string    `json:"path"`
	Format      string    `json:"format"`
	SizeBytes   int64     `json:"sizeBytes"`
}

// Entry is one file's most recent regression. It contains no file name or path.
type Entry struct {
	// FileKey identifies the file (a hash of its path). It is required and not
	// returned to the UI.
	FileKey     string    `json:"-"`
	FirstSeen   time.Time `json:"firstSeen"`
	CreatedAt   time.Time `json:"createdAt"`
	Occurrences int64     `json:"occurrences"`
	Operation   string    `json:"operation"`
	Label       string    `json:"label"`
	Detail      string    `json:"detail"`
	Format      string    `json:"format"`
	SizeBytes   int64     `json:"sizeBytes"`
	Codecs      string    `json:"codecs,omitempty"`
}

// Store is safe for concurrent use. The zero value is ready to use.
type Store struct {
	once sync.Once
	db   *dbsql.TempDB
	err  error
	// writeMu serializes Record so upserts and pruning do not interleave.
	writeMu sync.Mutex
}

func (s *Store) open() (*dbsql.TempDB, error) {
	s.once.Do(func() {
		db, err := dbsql.NewTempDB("shadow_regressions", &dbsql.TempDBConfig{
			PersistentFile: "shadow_regressions.db",
		})
		if err != nil {
			s.err = fmt.Errorf("open shadow regressions database: %w", err)
			return
		}
		if err := migrate(db); err != nil {
			db.Close()
			s.err = err
			return
		}
		s.db = db
	})
	return s.db, s.err
}

// migrate rebuilds the table when it has the earlier layout (one row per regression,
// no file_key). The table only holds dev-mode diagnostics, so the old rows are dropped.
func migrate(db *dbsql.TempDB) error {
	var hasFileKey int
	if err := db.DB().QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('shadow_regressions') WHERE name = 'file_key'`,
	).Scan(&hasFileKey); err != nil {
		return fmt.Errorf("inspect shadow regressions table: %w", err)
	}
	if hasFileKey == 0 {
		if _, err := db.Exec(`DROP TABLE IF EXISTS shadow_regressions`); err != nil {
			return fmt.Errorf("drop old shadow regressions table: %w", err)
		}
	}
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create shadow regressions table: %w", err)
	}
	if _, err := db.Exec(incompatibleSchema); err != nil {
		return fmt.Errorf("create shadow incompatible table: %w", err)
	}
	return nil
}

// RecordIncompatible stores a file go-media does not support that ffmpeg handled.
// A file that is already listed is updated in place and its count goes up by one.
func (s *Store) RecordIncompatible(e IncompatibleEntry) error {
	if e.FileKey == "" {
		return errors.New("incompatible file has no file key")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	db, err := s.open()
	if err != nil {
		return err
	}
	tx, err := db.DB().Begin()
	if err != nil {
		return err
	}
	// Rollback after a successful Commit returns sql.ErrTxDone, which is safe to ignore.
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`INSERT INTO shadow_incompatible
			(file_key, first_seen, created_at, operation, path, format, size_bytes, occurrences)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		 ON CONFLICT(file_key) DO UPDATE SET
			created_at = excluded.created_at,
			operation = excluded.operation,
			path = excluded.path,
			format = excluded.format,
			size_bytes = excluded.size_bytes,
			occurrences = occurrences + 1`,
		e.FileKey, e.CreatedAt.UnixMilli(), e.CreatedAt.UnixMilli(), e.Operation, e.Path, e.Format, e.SizeBytes,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM shadow_incompatible WHERE id NOT IN (
			SELECT id FROM shadow_incompatible ORDER BY created_at DESC, id DESC LIMIT ?)`,
		MaxEntries,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ListIncompatible returns one entry per file, most recently seen first.
func (s *Store) ListIncompatible() ([]IncompatibleEntry, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT first_seen, created_at, occurrences, operation, path, format, size_bytes
		 FROM shadow_incompatible ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []IncompatibleEntry{}
	for rows.Next() {
		var (
			firstMs, createdMs int64
			e                  IncompatibleEntry
		)
		if err := rows.Scan(&firstMs, &createdMs, &e.Occurrences, &e.Operation, &e.Path, &e.Format, &e.SizeBytes); err != nil {
			return nil, err
		}
		e.FirstSeen = time.UnixMilli(firstMs).UTC()
		e.CreatedAt = time.UnixMilli(createdMs).UTC()
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Record stores the latest regression for e's file. A file that already has a
// row is updated in place and its occurrence count goes up by one.
func (s *Store) Record(e Entry) error {
	if e.FileKey == "" {
		return errors.New("shadow regression has no file key")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	db, err := s.open()
	if err != nil {
		return err
	}
	// Use the raw *sql.DB rather than TempDB.BeginTransaction: that method takes
	// TempDB's internal mutex and expects the caller to release it, which this
	// package cannot do, so every later query would deadlock.
	tx, err := db.DB().Begin()
	if err != nil {
		return err
	}
	// Rollback after a successful Commit returns sql.ErrTxDone, which is safe to ignore.
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(
		`INSERT INTO shadow_regressions
			(file_key, first_seen, created_at, operation, label, detail, format, size_bytes, codecs, occurrences)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		 ON CONFLICT(file_key) DO UPDATE SET
			created_at = excluded.created_at,
			operation = excluded.operation,
			label = excluded.label,
			detail = excluded.detail,
			format = excluded.format,
			size_bytes = excluded.size_bytes,
			codecs = excluded.codecs,
			occurrences = occurrences + 1`,
		e.FileKey, e.CreatedAt.UnixMilli(), e.CreatedAt.UnixMilli(), e.Operation, e.Label, e.Detail,
		e.Format, e.SizeBytes, e.Codecs,
	); err != nil {
		return err
	}
	// Prune by last-seen time, so a file that keeps regressing is not dropped
	// just because its row was created long ago.
	if _, err := tx.Exec(
		`DELETE FROM shadow_regressions WHERE id NOT IN (
			SELECT id FROM shadow_regressions ORDER BY created_at DESC, id DESC LIMIT ?)`,
		MaxEntries,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// List returns one entry per file, most recently seen first.
func (s *Store) List() ([]Entry, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT first_seen, created_at, occurrences, operation, label, detail, format, size_bytes, codecs
		 FROM shadow_regressions ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var (
			firstMs, createdMs int64
			e                  Entry
		)
		if err := rows.Scan(&firstMs, &createdMs, &e.Occurrences, &e.Operation, &e.Label, &e.Detail,
			&e.Format, &e.SizeBytes, &e.Codecs); err != nil {
			return nil, err
		}
		e.FirstSeen = time.UnixMilli(firstMs).UTC()
		e.CreatedAt = time.UnixMilli(createdMs).UTC()
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Clear removes every stored regression and every incompatible file.
func (s *Store) Clear() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	db, err := s.open()
	if err != nil {
		return err
	}
	if _, err = db.Exec(`DELETE FROM shadow_regressions`); err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM shadow_incompatible`)
	return err
}
