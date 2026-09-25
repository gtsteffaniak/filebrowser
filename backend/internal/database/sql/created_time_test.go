package sql

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func TestCreatedTime_InsertAndReadBack(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(""))
	defer pop()

	db, _, err := NewIndexDB("created_time_set", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	created := time.Unix(1700000000, 0)
	info := &iteminfo.FileInfo{
		Path: "/a.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "a.txt",
			Size:    12,
			Created: &created,
			ModTime: time.Unix(1, 0),
			Type:    "text/plain",
		},
	}
	if insertErr := db.InsertItem("src1", "/a.txt", info); insertErr != nil {
		t.Fatal(insertErr)
	}

	got, err := db.GetItem("src1", "/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got.Created == nil {
		t.Fatal("expected non-nil Created")
	}
	if !got.Created.Equal(created.Truncate(time.Second)) {
		t.Errorf("Created = %v, want %v", got.Created, created.Truncate(time.Second))
	}
}

func TestCreatedTime_NilStaysNil(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(""))
	defer pop()

	db, _, err := NewIndexDB("created_time_nil", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	info := &iteminfo.FileInfo{
		Path: "/b.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "b.txt",
			Size:    12,
			ModTime: time.Unix(1, 0),
			Type:    "text/plain",
		},
	}
	if insertErr := db.InsertItem("src1", "/b.txt", info); insertErr != nil {
		t.Fatal(insertErr)
	}

	got, err := db.GetItem("src1", "/b.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got.Created != nil {
		t.Errorf("expected nil Created, got %v", got.Created)
	}
}

func TestCreatedTime_UpsertSetsCreated(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(""))
	defer pop()

	db, _, err := NewIndexDB("created_time_upsert", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	info := &iteminfo.FileInfo{
		Path: "/c.txt",
		ItemInfo: iteminfo.ItemInfo{
			Name:    "c.txt",
			Size:    12,
			ModTime: time.Unix(1, 0),
			Type:    "text/plain",
		},
	}
	if insertErr := db.InsertItem("src1", "/c.txt", info); insertErr != nil {
		t.Fatal(insertErr)
	}

	created := time.Unix(1700000000, 0)
	info.Created = &created
	if updateErr := db.InsertItem("src1", "/c.txt", info); updateErr != nil {
		t.Fatal(updateErr)
	}

	got, err := db.GetItem("src1", "/c.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got.Created == nil {
		t.Fatal("expected non-nil Created after upsert")
	}
	if !got.Created.Equal(created.Truncate(time.Second)) {
		t.Errorf("Created = %v, want %v", got.Created, created.Truncate(time.Second))
	}
}

func TestCreatedTime_MigrationAddsColumn(t *testing.T) {
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(""))
	defer pop()

	db, _, err := NewIndexDB("created_time_migration", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(`DROP TABLE index_items`); err != nil {
		t.Fatal(err)
	}
	oldSchema := `
	CREATE TABLE index_items (
		source TEXT NOT NULL,
		path TEXT NOT NULL,
		parent_path TEXT NOT NULL,
		name TEXT NOT NULL,
		size INTEGER NOT NULL,
		mod_time INTEGER NOT NULL,
		type TEXT NOT NULL,
		is_dir BOOLEAN NOT NULL,
		is_hidden BOOLEAN NOT NULL,
		has_preview BOOLEAN NOT NULL,
		last_updated INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (source, path)
	)`
	if _, err := db.Exec(oldSchema); err != nil {
		t.Fatal(err)
	}

	if err := db.CreateIndexTable(); err != nil {
		t.Fatal(err)
	}
	if !hasCreatedTimeColumn(t, db) {
		t.Fatal("expected created_time column after migration")
	}

	// Running again must not error (repeatable migration).
	if err := db.CreateIndexTable(); err != nil {
		t.Fatal(err)
	}
	if !hasCreatedTimeColumn(t, db) {
		t.Fatal("expected created_time column after second CreateIndexTable call")
	}
}

func hasCreatedTimeColumn(t *testing.T, db *IndexDB) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(index_items)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, columnType string
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "created_time" {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return false
}
