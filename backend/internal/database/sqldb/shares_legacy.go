package sqldb

import (
	"database/sql"
	"fmt"
)

const shareSelectColumns = `hash, user_id, source, path, expire, downloads,
			  password_hash, user_downloads, share_settings, version`

func sharesTableHasColumn(db *sql.DB, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(shares)`)
	if err != nil {
		return false, fmt.Errorf("read shares schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notnull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return false, fmt.Errorf("scan shares schema: %w", err)
		}
		if name == column {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate shares schema: %w", err)
	}
	return false, nil
}

// normalizeLegacyShareTokens keeps rollback compatibility with releases that still
// scan shares.token into a Go string (NULL is unsupported there).
func normalizeLegacyShareTokens(db *sql.DB) error {
	hasToken, err := sharesTableHasColumn(db, "token")
	if err != nil {
		return err
	}
	if !hasToken {
		return nil
	}
	if _, err := db.Exec(`UPDATE shares SET token = '' WHERE token IS NULL`); err != nil {
		return fmt.Errorf("normalize legacy share tokens: %w", err)
	}
	return nil
}

func (s *SQLStore) sharesHasLegacyTokenColumn() (bool, error) {
	return sharesTableHasColumn(s.db, "token")
}

func (s *SQLStore) legacyShareToken(hash string) (string, error) {
	hasToken, err := s.sharesHasLegacyTokenColumn()
	if err != nil {
		return "", err
	}
	if !hasToken {
		return "", nil
	}

	var token sql.NullString
	err = s.db.QueryRow(`SELECT token FROM shares WHERE hash = ?`, hash).Scan(&token)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read legacy share token: %w", err)
	}
	if !token.Valid {
		return "", nil
	}
	return token.String, nil
}
