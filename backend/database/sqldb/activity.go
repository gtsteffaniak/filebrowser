package sqldb

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// BulkInsertActivity inserts activity rows in a single transaction.
func (s *SQLStore) BulkInsertActivity(entries []activity.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin activity transaction: %w", err)
	}

	stmt, err := tx.Prepare(`INSERT INTO activity_log (
		created_at, user_id, event_type, source, path, target_path,
		ip_address, status, success, details
	) VALUES (?, ?, ?, ?, ?, ?, ?, 200, 1, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare activity insert: %w", err)
	}
	defer stmt.Close()

	for _, e := range entries {
		detailsJSON, err := activity.MarshalDetailsJSON(e.Details)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("marshal activity details: %w", err)
		}
		_, err = stmt.Exec(
			e.CreatedAt,
			shareUserIDDB(e.UserID),
			string(e.EventType),
			nullString(e.Source),
			nullString(e.Path),
			nullString(e.TargetPath),
			nullString(e.IPAddress),
			detailsJSON,
		)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert activity row: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit activity transaction: %w", err)
	}
	return nil
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// CountActivity returns the total matching rows for pagination.
func (s *SQLStore) CountActivity(filter activity.QueryFilter) (int, error) {
	where, args := buildActivityWhere(filter)
	query := "SELECT COUNT(*) FROM activity_log" + where
	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count activity: %w", err)
	}
	return count, nil
}

// ListActivity returns paginated activity rows newest first with actor usernames joined.
func (s *SQLStore) ListActivity(filter activity.QueryFilter) ([]activity.ListRow, error) {
	where, args := buildActivityWhere(filter)
	where = whereWithAlias(where, "a")
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	page := filter.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * limit

	query := `SELECT a.id, a.created_at, a.user_id, a.event_type, a.source, a.path, a.target_path,
		a.ip_address, a.details,
		COALESCE(u.username, '') AS actor_username
		FROM activity_log a
		LEFT JOIN users u ON a.user_id = u.user_id` + where +
		" ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list activity: %w", err)
	}
	defer rows.Close()

	var items []activity.ListRow
	for rows.Next() {
		row, err := scanActivityListRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan activity row: %w", err)
		}
		items = append(items, *row)
	}
	return utils.NonNilSlice(items), rows.Err()
}

func scanActivityListRow(scanner interface {
	Scan(dest ...interface{}) error
}) (*activity.ListRow, error) {
	var e activity.Entry
	var userIDStr string
	var source, path, targetPath, ipAddress sql.NullString
	var detailsJSON string
	var actorUsername string

	err := scanner.Scan(
		&e.ID,
		&e.CreatedAt,
		&userIDStr,
		&e.EventType,
		&source,
		&path,
		&targetPath,
		&ipAddress,
		&detailsJSON,
		&actorUsername,
	)
	if err != nil {
		return nil, err
	}

	if err := scanShareUserID(userIDStr, &e.UserID); err != nil {
		return nil, err
	}
	if source.Valid {
		e.Source = source.String
	}
	if path.Valid {
		e.Path = path.String
	}
	if targetPath.Valid {
		e.TargetPath = targetPath.String
	}
	if ipAddress.Valid {
		e.IPAddress = ipAddress.String
	}
	if err := activity.UnmarshalDetailsJSON(detailsJSON, &e.Details); err != nil {
		return nil, fmt.Errorf("unmarshal activity details: %w", err)
	}
	return &activity.ListRow{Entry: e, ActorUsername: actorUsername}, nil
}

// ListActivityStats returns aggregated counts for charts.
func (s *SQLStore) ListActivityStats(filter activity.QueryFilter) ([]activity.StatsBucket, error) {
	interval := normalizeActivityInterval(filter)
	splitBy := normalizeActivitySplitBy(filter)
	hasTimeBucket := interval != "none"

	table := "activity_log"
	where, args := buildActivityWhereTable(filter, table)
	bucketExpr := activityBucketExpr(interval)

	var query string
	switch {
	case hasTimeBucket && splitBy == "eventType":
		query = fmt.Sprintf(`SELECT %s AS bucket, event_type AS series_key, '' AS series_label, COUNT(*) AS cnt
			FROM %s%s GROUP BY bucket, event_type ORDER BY bucket ASC, cnt DESC`, bucketExpr, table, where)
	case hasTimeBucket && splitBy == "user":
		query = fmt.Sprintf(`SELECT %s AS bucket, a.user_id AS series_key, COALESCE(u.username, CASE WHEN a.user_id = '0' THEN '%s' ELSE a.user_id END) AS series_label, COUNT(*) AS cnt
			FROM activity_log a LEFT JOIN users u ON a.user_id = u.user_id%s
			GROUP BY bucket, a.user_id ORDER BY bucket ASC, cnt DESC`, bucketExpr, users.AnonymousUserName, whereWithAlias(where, "a"))
	case hasTimeBucket && splitBy == "none":
		query = fmt.Sprintf(`SELECT %s AS bucket, 'total' AS series_key, '' AS series_label, COUNT(*) AS cnt
			FROM %s%s GROUP BY bucket ORDER BY bucket ASC`, bucketExpr, table, where)
	case !hasTimeBucket && splitBy == "eventType":
		query = fmt.Sprintf(`SELECT 0 AS bucket, event_type AS series_key, '' AS series_label, COUNT(*) AS cnt
			FROM %s%s GROUP BY event_type ORDER BY cnt DESC`, table, where)
	case !hasTimeBucket && splitBy == "user":
		query = fmt.Sprintf(`SELECT 0 AS bucket, a.user_id AS series_key, COALESCE(u.username, CASE WHEN a.user_id = '0' THEN '%s' ELSE a.user_id END) AS series_label, COUNT(*) AS cnt
			FROM activity_log a LEFT JOIN users u ON a.user_id = u.user_id%s
			GROUP BY a.user_id ORDER BY cnt DESC`, users.AnonymousUserName, whereWithAlias(where, "a"))
	default:
		query = fmt.Sprintf(`SELECT 0 AS bucket, 'total' AS series_key, '' AS series_label, COUNT(*) AS cnt
			FROM %s%s`, table, where)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list activity stats: %w", err)
	}
	defer rows.Close()

	var buckets []activity.StatsBucket
	for rows.Next() {
		var b activity.StatsBucket
		if err := rows.Scan(&b.Bucket, &b.SeriesKey, &b.SeriesLabel, &b.Count); err != nil {
			return nil, fmt.Errorf("scan activity stats: %w", err)
		}
		if splitBy == "eventType" {
			b.EventType = b.SeriesKey
		}
		buckets = append(buckets, b)
	}
	return utils.NonNilSlice(buckets), rows.Err()
}

func normalizeActivityInterval(filter activity.QueryFilter) string {
	if filter.Interval != "" {
		return filter.Interval
	}
	switch filter.GroupBy {
	case "day":
		return "day"
	case "none":
		return "none"
	case "hour", "":
		return "hour"
	default:
		return "hour"
	}
}

func normalizeActivitySplitBy(filter activity.QueryFilter) string {
	if filter.SplitBy != "" {
		return filter.SplitBy
	}
	return "eventType"
}

func activityBucketExpr(interval string) string {
	switch interval {
	case "minute":
		return "((created_at / 60) * 60)"
	case "hour":
		return "((created_at / 3600) * 3600)"
	case "day":
		return "((created_at / 86400) * 86400)"
	default:
		return "0"
	}
}

func whereWithAlias(where, alias string) string {
	if where == "" {
		return ""
	}
	return strings.ReplaceAll(where, "activity_log.", alias+".")
}

// PurgeActivityBefore deletes rows older than cutoffUnix.
func (s *SQLStore) PurgeActivityBefore(cutoffUnix int64) (int64, error) {
	res, err := s.db.Exec("DELETE FROM activity_log WHERE created_at < ?", cutoffUnix)
	if err != nil {
		return 0, fmt.Errorf("purge activity: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge activity rows affected: %w", err)
	}
	return n, nil
}

func buildActivityWhere(filter activity.QueryFilter) (string, []interface{}) {
	return buildActivityWhereTable(filter, "activity_log")
}

func buildActivityWhereTable(filter activity.QueryFilter, table string) (string, []interface{}) {
	var clauses []string
	var args []interface{}
	col := func(name string) string {
		return table + "." + name
	}

	if filter.From > 0 {
		clauses = append(clauses, col("created_at")+" >= ?")
		args = append(args, filter.From)
	}
	if filter.To > 0 {
		clauses = append(clauses, col("created_at")+" <= ?")
		args = append(args, filter.To)
	}
	if filter.UserFilter {
		clauses = append(clauses, col("user_id")+" = ?")
		args = append(args, shareUserIDDB(filter.UserID))
	}
	if filter.ShareOwnerFilter {
		appendShareOwnerActivityClauses(&clauses, &args, filter, table)
	}
	if filter.Scope == "shares" {
		appendShareScopeEventClauses(&clauses, &args, filter, table)
	} else if len(filter.EventTypes) > 0 {
		placeholders := make([]string, len(filter.EventTypes))
		for i, et := range filter.EventTypes {
			placeholders[i] = "?"
			args = append(args, string(et))
		}
		clauses = append(clauses, col("event_type")+" IN ("+strings.Join(placeholders, ",")+")")
	}
	if filter.Source != "" {
		clauses = append(clauses, col("source")+" = ?")
		args = append(args, filter.Source)
	}
	if filter.PathPrefix != "" {
		like := escapeLikePrefix(filter.PathPrefix)
		clauses = append(clauses, "("+col("path")+" LIKE ? ESCAPE '\\' OR EXISTS (SELECT 1 FROM json_each("+table+".details, '$.paths') je WHERE je.value LIKE ? ESCAPE '\\'))")
		args = append(args, like, like)
	}
	if filter.PathGlob != "" {
		clauses = append(clauses, "("+col("path")+" GLOB ? OR EXISTS (SELECT 1 FROM json_each("+table+".details, '$.paths') je WHERE je.value GLOB ?))")
		args = append(args, filter.PathGlob, filter.PathGlob)
	}
	if filter.ShareHash != "" {
		clauses = append(clauses, "json_extract("+table+".details, '$.shareHash') = ?")
		args = append(args, filter.ShareHash)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	return where, args
}

func shareHashPresentExpr(table string) string {
	return "COALESCE(json_extract(" + table + ".details, '$.shareHash'), '') != ''"
}

func appendShareScopeEventClauses(clauses *[]string, args *[]interface{}, filter activity.QueryFilter, table string) {
	col := func(name string) string {
		return table + "." + name
	}
	shareHash := shareHashPresentExpr(table)
	legacyShareDL := col("event_type") + " = ?"
	legacyArg := string(activity.EventShareDownload)

	if len(filter.EventTypes) == 0 {
		*clauses = append(*clauses, "("+col("event_type")+" IN (?,?,?) OR ("+col("event_type")+" = ? AND "+shareHash+") OR "+legacyShareDL+")")
		*args = append(*args,
			string(activity.EventShareCreate),
			string(activity.EventShareUpdate),
			string(activity.EventShareDelete),
			string(activity.EventDownload),
			legacyArg,
		)
		return
	}

	var parts []string
	for _, et := range filter.EventTypes {
		switch et {
		case activity.EventShareCreate, activity.EventShareUpdate, activity.EventShareDelete:
			parts = append(parts, col("event_type")+" = ?")
			*args = append(*args, string(et))
		case activity.EventDownload:
			parts = append(parts, "("+col("event_type")+" = ? AND "+shareHash+")")
			*args = append(*args, string(et))
			parts = append(parts, legacyShareDL)
			*args = append(*args, legacyArg)
		}
	}
	if len(parts) > 0 {
		*clauses = append(*clauses, "("+strings.Join(parts, " OR ")+")")
	}
}

func appendShareOwnerActivityClauses(clauses *[]string, args *[]interface{}, filter activity.QueryFilter, table string) {
	col := func(name string) string {
		return table + "." + name
	}
	ownerID := shareUserIDDB(filter.ShareOwnerUserID)
	shareOwnerInDetails := "json_extract(" + table + ".details, '$.shareOwnerUserId') = ?"

	downloadMatch := "(" + col("event_type") + " IN (?,?) AND (" + shareOwnerInDetails
	downloadArgs := []interface{}{
		string(activity.EventDownload),
		string(activity.EventShareDownload),
		ownerID,
	}
	if len(filter.OwnedShareHashes) > 0 {
		placeholders := make([]string, len(filter.OwnedShareHashes))
		for i, h := range filter.OwnedShareHashes {
			placeholders[i] = "?"
			downloadArgs = append(downloadArgs, h)
		}
		downloadMatch += " OR json_extract(" + table + ".details, '$.shareHash') IN (" + strings.Join(placeholders, ",") + ")"
	}
	downloadMatch += "))"

	clause := "((" + col("event_type") + " IN (?,?,?) AND " + col("user_id") + " = ?) OR " + downloadMatch + ")"
	*clauses = append(*clauses, clause)
	*args = append(*args,
		string(activity.EventShareCreate),
		string(activity.EventShareUpdate),
		string(activity.EventShareDelete),
		ownerID,
	)
	*args = append(*args, downloadArgs...)
}

func escapeLikePrefix(prefix string) string {
	var b strings.Builder
	b.Grow(len(prefix) + 8)
	for _, r := range prefix {
		switch r {
		case '\\', '%', '_':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('%')
	return b.String()
}

