package sqldb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/quota"
)

func scanQuotaUserID(s string, dest *uint64) error {
	if s == "" {
		*dest = 0
		return nil
	}
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("parse quota user_id: %w", err)
	}
	*dest = u
	return nil
}

func quotaUserIDDB(id uint64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatUint(id, 10)
}

// GetAllFolderQuotas loads every folder quota row.
func (s *SQLStore) GetAllFolderQuotas() ([]quota.FolderQuota, error) {
	rows, err := s.db.Query(`SELECT id, source, path, user_id, limit_bytes, meter FROM quotas`)
	if err != nil {
		return nil, fmt.Errorf("query quotas: %w", err)
	}
	defer rows.Close()

	var out []quota.FolderQuota
	for rows.Next() {
		var q quota.FolderQuota
		var userIDStr string
		if err := rows.Scan(&q.ID, &q.Source, &q.Path, &userIDStr, &q.LimitBytes, &q.Meter); err != nil {
			return nil, fmt.Errorf("scan quota: %w", err)
		}
		if err := scanQuotaUserID(userIDStr, &q.UserID); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// GetFolderQuotaByID returns one folder quota.
func (s *SQLStore) GetFolderQuotaByID(id string) (*quota.FolderQuota, error) {
	var q quota.FolderQuota
	var userIDStr string
	err := s.db.QueryRow(`SELECT id, source, path, user_id, limit_bytes, meter FROM quotas WHERE id = ?`, id).
		Scan(&q.ID, &q.Source, &q.Path, &userIDStr, &q.LimitBytes, &q.Meter)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("quota not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get quota: %w", err)
	}
	if err := scanQuotaUserID(userIDStr, &q.UserID); err != nil {
		return nil, err
	}
	return &q, nil
}

// GetFolderQuotasForSourcePath returns quotas for a source and optional path prefix.
func (s *SQLStore) GetFolderQuotasForSourcePath(source, path string) ([]quota.FolderQuota, error) {
	path = strings.TrimSuffix(path, "/")
	query := `SELECT id, source, path, user_id, limit_bytes, meter FROM quotas WHERE source = ?`
	args := []interface{}{source}
	if path != "" {
		query += ` AND (path = ? OR path LIKE ?)`
		args = append(args, path, path+"/%")
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query quotas for source: %w", err)
	}
	defer rows.Close()

	var out []quota.FolderQuota
	for rows.Next() {
		var q quota.FolderQuota
		var userIDStr string
		if err := rows.Scan(&q.ID, &q.Source, &q.Path, &userIDStr, &q.LimitBytes, &q.Meter); err != nil {
			return nil, fmt.Errorf("scan quota: %w", err)
		}
		if err := scanQuotaUserID(userIDStr, &q.UserID); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// SaveFolderQuota inserts or updates a folder quota.
func (s *SQLStore) SaveFolderQuota(q *quota.FolderQuota) error {
	meter := q.Meter
	if meter == "" {
		meter = quota.MeterIndexSize
	}
	_, err := s.db.Exec(
		`INSERT INTO quotas (id, source, path, user_id, limit_bytes, meter, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET source=excluded.source, path=excluded.path, user_id=excluded.user_id, limit_bytes=excluded.limit_bytes, meter=excluded.meter`,
		q.ID, q.Source, q.Path, quotaUserIDDB(q.UserID), q.LimitBytes, meter, currentTimestamp(),
	)
	if err != nil {
		return fmt.Errorf("save quota: %w", err)
	}
	return nil
}

// DeleteFolderQuota removes a folder quota and its counter.
func (s *SQLStore) DeleteFolderQuota(id string) error {
	if _, err := s.db.Exec(`DELETE FROM quotas WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete quota: %w", err)
	}
	if _, err := s.db.Exec(`DELETE FROM quota_counters WHERE quota_id = ?`, id); err != nil {
		return fmt.Errorf("delete quota counter: %w", err)
	}
	return nil
}

// GetAllQuotaCounters loads all quota counters.
func (s *SQLStore) GetAllQuotaCounters() ([]quota.Counter, error) {
	rows, err := s.db.Query(`SELECT quota_id, used_bytes, reserved_bytes FROM quota_counters`)
	if err != nil {
		return nil, fmt.Errorf("query quota counters: %w", err)
	}
	defer rows.Close()

	var out []quota.Counter
	for rows.Next() {
		var c quota.Counter
		if err := rows.Scan(&c.QuotaID, &c.UsedBytes, &c.ReservedBytes); err != nil {
			return nil, fmt.Errorf("scan quota counter: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetQuotaCounter returns one counter row.
func (s *SQLStore) GetQuotaCounter(quotaID string) (*quota.Counter, error) {
	var c quota.Counter
	err := s.db.QueryRow(`SELECT quota_id, used_bytes, reserved_bytes FROM quota_counters WHERE quota_id = ?`, quotaID).
		Scan(&c.QuotaID, &c.UsedBytes, &c.ReservedBytes)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("quota counter not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get quota counter: %w", err)
	}
	return &c, nil
}

// EnsureQuotaCounter creates a counter row if missing.
func (s *SQLStore) EnsureQuotaCounter(quotaID string) error {
	_, err := s.db.Exec(
		`INSERT INTO quota_counters (quota_id, used_bytes, reserved_bytes, version, updated_at)
		 VALUES (?, 0, 0, 0, ?)
		 ON CONFLICT(quota_id) DO NOTHING`,
		quotaID, currentTimestamp(),
	)
	if err != nil {
		return fmt.Errorf("ensure quota counter: %w", err)
	}
	return nil
}

// DeleteQuotaCounter removes a counter row.
func (s *SQLStore) DeleteQuotaCounter(quotaID string) error {
	_, err := s.db.Exec(`DELETE FROM quota_counters WHERE quota_id = ?`, quotaID)
	if err != nil {
		return fmt.Errorf("delete quota counter: %w", err)
	}
	return nil
}

// BulkUpsertQuotaCounters persists dirty counters.
func (s *SQLStore) BulkUpsertQuotaCounters(counters []quota.Counter) error {
	if len(counters) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin quota counter tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.Prepare(
		`INSERT INTO quota_counters (quota_id, used_bytes, reserved_bytes, version, updated_at)
		 VALUES (?, ?, ?, 0, ?)
		 ON CONFLICT(quota_id) DO UPDATE SET
		   used_bytes = excluded.used_bytes,
		   reserved_bytes = excluded.reserved_bytes,
		   updated_at = excluded.updated_at`,
	)
	if err != nil {
		return fmt.Errorf("prepare quota counter upsert: %w", err)
	}
	defer stmt.Close()

	now := currentTimestamp()
	for _, c := range counters {
		if _, err := stmt.Exec(c.QuotaID, c.UsedBytes, c.ReservedBytes, now); err != nil {
			return fmt.Errorf("upsert quota counter %s: %w", c.QuotaID, err)
		}
	}
	return tx.Commit()
}
