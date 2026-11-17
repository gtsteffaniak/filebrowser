// Package shadowregress stores significant go-media vs ffmpeg regressions found
// by dev-mode shadow runs, so admins can review them in the settings UI.
//
// The database is a separate SQLite file in the cache directory's sql/ folder,
// created the same way as the index database. It is opened lazily, so nothing
// is created until the first regression is recorded or the viewer is opened.
package shadowregress

import (
	"fmt"
	"sync"
	"time"

	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
)

// MaxEntries is the most recent regressions kept. Older rows are pruned on insert.
const MaxEntries = 1000

const schema = `
CREATE TABLE IF NOT EXISTS shadow_regressions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at INTEGER NOT NULL,
	operation TEXT NOT NULL,
	label TEXT NOT NULL,
	detail TEXT NOT NULL,
	format TEXT NOT NULL,
	size_bytes INTEGER NOT NULL,
	codecs TEXT NOT NULL DEFAULT ''
);`

// Entry is one recorded regression. It contains no file name or path.
type Entry struct {
	CreatedAt time.Time `json:"createdAt"`
	Operation string    `json:"operation"`
	Label     string    `json:"label"`
	Detail    string    `json:"detail"`
	Format    string    `json:"format"`
	SizeBytes int64     `json:"sizeBytes"`
	Codecs    string    `json:"codecs,omitempty"`
}

// Store is safe for concurrent use. The zero value is ready to use.
type Store struct {
	once sync.Once
	db   *dbsql.TempDB
	err  error
	// writeMu serializes Record so inserts and pruning do not interleave.
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
		if _, err := db.Exec(schema); err != nil {
			db.Close()
			s.err = fmt.Errorf("create shadow regressions table: %w", err)
			return
		}
		s.db = db
	})
	return s.db, s.err
}

// Record stores one regression and prunes the table to MaxEntries rows.
func (s *Store) Record(e Entry) error {
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
	defer tx.Rollback()

	if _, err := tx.Exec(
		`INSERT INTO shadow_regressions (created_at, operation, label, detail, format, size_bytes, codecs)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.CreatedAt.UnixMilli(), e.Operation, e.Label, e.Detail, e.Format, e.SizeBytes, e.Codecs,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`DELETE FROM shadow_regressions WHERE id NOT IN (
			SELECT id FROM shadow_regressions ORDER BY id DESC LIMIT ?)`,
		MaxEntries,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// List returns all stored regressions, newest first.
func (s *Store) List() ([]Entry, error) {
	db, err := s.open()
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT created_at, operation, label, detail, format, size_bytes, codecs
		 FROM shadow_regressions ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []Entry{}
	for rows.Next() {
		var (
			createdMs int64
			e         Entry
		)
		if err := rows.Scan(&createdMs, &e.Operation, &e.Label, &e.Detail, &e.Format, &e.SizeBytes, &e.Codecs); err != nil {
			return nil, err
		}
		e.CreatedAt = time.UnixMilli(createdMs).UTC()
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Clear removes every stored regression.
func (s *Store) Clear() error {
	db, err := s.open()
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM shadow_regressions`)
	return err
}
