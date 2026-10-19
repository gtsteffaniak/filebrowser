package sqldb

import (
	"database/sql"
	"fmt"
)

const accessSourceModeTable = `
CREATE TABLE IF NOT EXISTS access_source_mode (
	source TEXT PRIMARY KEY NOT NULL,
	case_insensitive INTEGER NOT NULL
);
`

func migrateV5AccessSourceMode(db *sql.DB) error {
	if _, err := db.Exec(accessSourceModeTable); err != nil {
		return fmt.Errorf("migrate v5 access_source_mode: %w", err)
	}
	return nil
}

// GetAccessSourceCaseMode returns persisted case mode for a source.
func (s *SQLStore) GetAccessSourceCaseMode(source string) (insensitive bool, found bool, err error) {
	var v int
	err = s.db.QueryRow(`SELECT case_insensitive FROM access_source_mode WHERE source = ?`, source).Scan(&v)
	if err == sql.ErrNoRows {
		return false, false, nil
	}
	if err != nil {
		return false, false, fmt.Errorf("get access source mode: %w", err)
	}
	return v != 0, true, nil
}

// SaveAccessSourceCaseMode persists case mode for a source.
func (s *SQLStore) SaveAccessSourceCaseMode(source string, insensitive bool) error {
	v := 0
	if insensitive {
		v = 1
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO access_source_mode (source, case_insensitive) VALUES (?, ?)`, source, v)
	if err != nil {
		return fmt.Errorf("save access source mode: %w", err)
	}
	return nil
}
