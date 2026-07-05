package state

import (
	"fmt"
	"strconv"

	activityrec "github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// InitActivityRecorder starts the global activity recorder using the SQL store.
func InitActivityRecorder(cfg settings.Database) {
	if sqlDb == nil {
		return
	}
	activityrec.Initialize(sqlDb, cfg)
}

// StopActivityRecorder flushes pending activity and stops the recorder.
func StopActivityRecorder() {
	activityrec.Stop()
}

// RecordActivity appends an activity entry to the buffer.
func RecordActivity(entry activitydb.Entry) {
	activityrec.Record(entry)
}

// ListActivity returns paginated activity rows with total count.
func ListActivity(filter activitydb.QueryFilter) ([]activitydb.FrontendEntry, int, error) {
	if sqlDb == nil {
		return nil, 0, fmt.Errorf("sql store not initialized")
	}
	total, err := sqlDb.CountActivity(filter)
	if err != nil {
		return nil, 0, err
	}
	rows, err := sqlDb.ListActivity(filter)
	if err != nil {
		return nil, 0, err
	}
	items := make([]activitydb.FrontendEntry, 0, len(rows))
	for _, row := range rows {
		username := activityActorUsername(row.Entry, row.ActorUsername)
		items = append(items, row.PrepForFrontend(username))
	}
	return items, total, nil
}

func activityActorUsername(row activitydb.Entry, joinedUsername string) string {
	if row.UserID == 0 {
		return users.AnonymousUserName
	}
	if joinedUsername != "" {
		return joinedUsername
	}
	if u, err := GetUserByID(row.UserID); err == nil {
		return u.Username
	}
	return strconv.FormatUint(row.UserID, 10)
}

// ListActivityStats returns aggregated activity buckets for charts.
func ListActivityStats(filter activitydb.QueryFilter) ([]activitydb.StatsBucket, error) {
	if sqlDb == nil {
		return nil, fmt.Errorf("sql store not initialized")
	}
	return sqlDb.ListActivityStats(filter)
}
