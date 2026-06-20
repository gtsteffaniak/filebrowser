package sqldb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/database/activity"
)

func TestActivityBulkInsertAndQuery(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")

	store, _, err := NewSQLStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	entries := []activitydb.Entry{
		{
			CreatedAt: now,
			UserID:    42,
			EventType: activitydb.EventDownload,
			Source:    "default",
			Path:      "/file.txt",
			Status:    200,
			Success:   true,
			Details:   activitydb.Details{Source: "default", Path: "/file.txt"},
		},
		{
			CreatedAt: now - 10,
			UserID:    99,
			EventType: activitydb.EventUpload,
			Source:    "default",
			Path:      "/upload.bin",
			Status:    200,
			Success:   true,
		},
	}
	if err = store.BulkInsertActivity(entries); err != nil {
		t.Fatalf("BulkInsertActivity: %v", err)
	}

	filter := activitydb.QueryFilter{
		From:       now - 3600,
		To:         now + 10,
		UserID:     42,
		UserFilter: true,
		Page:       1,
		Limit:      10,
	}
	count, err := store.CountActivity(filter)
	if err != nil {
		t.Fatalf("CountActivity: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	rows, err := store.ListActivity(filter)
	if err != nil {
		t.Fatalf("ListActivity: %v", err)
	}
	if len(rows) != 1 || rows[0].EventType != activitydb.EventDownload {
		t.Fatalf("unexpected rows: %+v", rows)
	}

	anonEntry := activitydb.Entry{
		CreatedAt: now - 5,
		UserID:    0,
		EventType: activitydb.EventDownload,
		Source:    "default",
		Path:      "/shared.txt",
		Status:    200,
		Success:   true,
		Details:   activitydb.Details{ShareHash: "test-hash", Source: "default", Path: "/shared.txt"},
	}
	if err = store.BulkInsertActivity([]activitydb.Entry{anonEntry}); err != nil {
		t.Fatalf("BulkInsertActivity anonymous: %v", err)
	}

	anonFilter := activitydb.QueryFilter{
		From:       now - 3600,
		To:         now + 10,
		UserID:     0,
		UserFilter: true,
		Page:       1,
		Limit:      10,
	}
	anonCount, err := store.CountActivity(anonFilter)
	if err != nil {
		t.Fatalf("CountActivity anonymous: %v", err)
	}
	if anonCount != 1 {
		t.Fatalf("expected anonymous count 1, got %d", anonCount)
	}

	stats, err := store.ListActivityStats(activitydb.QueryFilter{
		From:     now - 3600,
		To:       now + 10,
		Interval: "hour",
		SplitBy:  "eventType",
	})
	if err != nil {
		t.Fatalf("ListActivityStats: %v", err)
	}
	if len(stats) < 2 {
		t.Fatalf("expected stats buckets, got %+v", stats)
	}

	n, err := store.PurgeActivityBefore(now + 1)
	if err != nil {
		t.Fatalf("PurgeActivityBefore: %v", err)
	}
	if n < 2 {
		t.Fatalf("expected purge >= 2, got %d", n)
	}

	// Ensure DB file exists on disk
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file missing: %v", err)
	}
}

func TestActivityShareScope(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "share-scope.sqlite")

	store, _, err := NewSQLStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	entries := []activitydb.Entry{
		{
			CreatedAt: now,
			UserID:    1,
			EventType: activitydb.EventShareCreate,
			Source:    "default",
			Path:      "/share-path",
			Status:    200,
			Success:   true,
			Details:   activitydb.Details{ShareHash: "abc123"},
		},
		{
			CreatedAt: now,
			UserID:    0,
			EventType: activitydb.EventDownload,
			Source:    "default",
			Path:      "/file.txt",
			Status:    200,
			Success:   true,
			Details:   activitydb.Details{ShareHash: "abc123", Source: "default", Path: "/file.txt"},
		},
		{
			CreatedAt: now,
			UserID:    0,
			EventType: activitydb.EventDownload,
			Source:    "default",
			Path:      "/plain.txt",
			Status:    200,
			Success:   true,
		},
		{
			CreatedAt: now,
			UserID:    0,
			EventType: activitydb.EventShareDownload,
			Source:    "default",
			Path:      "/legacy.txt",
			Status:    200,
			Success:   true,
			Details:   activitydb.Details{ShareHash: "legacy"},
		},
	}
	if err = store.BulkInsertActivity(entries); err != nil {
		t.Fatalf("BulkInsertActivity: %v", err)
	}

	base := activitydb.QueryFilter{
		From:  now - 10,
		To:    now + 10,
		Scope: "shares",
		Page:  1,
		Limit: 50,
	}
	shareCount, err := store.CountActivity(base)
	if err != nil {
		t.Fatalf("CountActivity shares scope: %v", err)
	}
	if shareCount != 3 {
		t.Fatalf("shares scope expected 3 rows (create + share download + legacy), got %d", shareCount)
	}

	filesFilter := activitydb.QueryFilter{
		From:       now - 10,
		To:         now + 10,
		Scope:      "files",
		EventTypes: activitydb.FileEventTypes,
		Page:       1,
		Limit:      50,
	}
	fileCount, err := store.CountActivity(filesFilter)
	if err != nil {
		t.Fatalf("CountActivity files scope: %v", err)
	}
	if fileCount != 2 {
		t.Fatalf("files scope expected 2 download rows, got %d", fileCount)
	}

	downloadOnly := base
	downloadOnly.EventTypes = []activitydb.EventType{activitydb.EventDownload}
	dlCount, err := store.CountActivity(downloadOnly)
	if err != nil {
		t.Fatalf("CountActivity shares download filter: %v", err)
	}
	if dlCount != 2 {
		t.Fatalf("shares download filter expected 2 rows, got %d", dlCount)
	}
}

func TestActivityShareOwnerFilter(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "share-owner.sqlite")

	store, _, err := NewSQLStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLStore: %v", err)
	}
	defer store.Close()

	now := time.Now().Unix()
	entries := []activitydb.Entry{
		{
			CreatedAt: now,
			UserID:    5,
			EventType: activitydb.EventShareCreate,
			Source:    "default",
			Path:      "/mine",
			Status:    200,
			Success:   true,
			Details:   activitydb.Details{ShareHash: "owned-hash"},
		},
		{
			CreatedAt: now,
			UserID:    0,
			EventType: activitydb.EventDownload,
			Source:    "default",
			Path:      "/file.txt",
			Status:    200,
			Success:   true,
			Details: activitydb.Details{
				ShareHash:        "owned-hash",
				ShareOwnerUserID: 5,
				Source:           "default",
				Path:             "/file.txt",
			},
		},
		{
			CreatedAt: now,
			UserID:    9,
			EventType: activitydb.EventShareCreate,
			Source:    "default",
			Path:      "/other",
			Status:    200,
			Success:   true,
			Details:   activitydb.Details{ShareHash: "other-hash"},
		},
	}
	if err = store.BulkInsertActivity(entries); err != nil {
		t.Fatalf("BulkInsertActivity: %v", err)
	}

	ownerFilter := activitydb.QueryFilter{
		From:             now - 10,
		To:               now + 10,
		Scope:            "shares",
		ShareOwnerUserID: 5,
		ShareOwnerFilter: true,
		OwnedShareHashes: []string{"owned-hash"},
		Page:             1,
		Limit:            50,
	}
	ownerCount, err := store.CountActivity(ownerFilter)
	if err != nil {
		t.Fatalf("CountActivity share owner: %v", err)
	}
	if ownerCount != 2 {
		t.Fatalf("share owner filter expected 2 rows, got %d", ownerCount)
	}
}
