package sql

import (
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func newFullTextTestDB(t *testing.T) *IndexDB {
	t.Helper()
	t.Cleanup(pushTestIndexConfig(t, t.TempDir(), testIndexSQLConfig(settingsOff)))
	db, _, err := NewIndexDB("fts_test", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatalf("NewIndexDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

const settingsOff = "off"

func body(s string) *string { return &s }

func TestFullTextSearchMatchesContentAndName(t *testing.T) {

	db := newFullTextTestDB(t)
	items := []*iteminfo.FileInfo{
		{Path: "/notes/annual report.md", ItemInfo: iteminfo.ItemInfo{Name: "annual report.md", ModTime: time.Unix(100, 0), Type: "text/markdown"}, FullTextBody: body("annual report.md\nbudget numbers for the quarter")},
		{Path: "/notes/other.txt", ItemInfo: iteminfo.ItemInfo{Name: "other.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("other.txt\nunrelated content")},
		{Path: "/docs/budget.txt", ItemInfo: iteminfo.ItemInfo{Name: "budget.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("budget.txt\nplain body")},
	}
	if err := db.BulkInsertItems("src", items); err != nil {
		t.Fatalf("BulkInsertItems: %v", err)
	}

	paths, err := db.SearchFullText("src", "/", []string{"budget"}, 100)
	if err != nil {
		t.Fatalf("SearchFullText: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("want 2 content hits, got %v", paths)
	}

	// Scope restricts to the given folder prefix.
	scoped, err := db.SearchFullText("src", "/notes/", []string{"budget"}, 100)
	if err != nil {
		t.Fatalf("SearchFullText scoped: %v", err)
	}
	if len(scoped) != 1 || scoped[0] != "/notes/annual report.md" {
		t.Fatalf("scoped hits = %v", scoped)
	}

	// Every term must match (AND).
	both, err := db.SearchFullText("src", "/", []string{"budget", "quarter"}, 100)
	if err != nil {
		t.Fatalf("SearchFullText AND: %v", err)
	}
	if len(both) != 1 || both[0] != "/notes/annual report.md" {
		t.Fatalf("AND hits = %v", both)
	}
}

func TestFullTextUpsertReplacesAndDeletesRemove(t *testing.T) {
	db := newFullTextTestDB(t)
	item := &iteminfo.FileInfo{Path: "/a.txt", ItemInfo: iteminfo.ItemInfo{Name: "a.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("a.txt\noldword")}
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{item}); err != nil {
		t.Fatal(err)
	}

	item.FullTextBody = body("a.txt\nnewword")
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{item}); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"oldword"}, 10); len(hits) != 0 {
		t.Fatalf("stale body still matches: %v", hits)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"newword"}, 10); len(hits) != 1 {
		t.Fatalf("updated body not searchable: %v", hits)
	}

	// Updating without a body clears the content row (name still searchable).
	item.FullTextBody = nil
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{item}); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"newword"}, 10); len(hits) != 0 {
		t.Fatalf("body not cleared on nil: %v", hits)
	}

	if err := db.DeleteItem("src", "/a.txt", false); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := db.QueryRow(`SELECT count(*) FROM index_fts WHERE source = 'src'`).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatalf("fts rows left after delete: %d", remaining)
	}
}

func TestFullTextOrphansArePurged(t *testing.T) {
	db := newFullTextTestDB(t)
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{
		{Path: "/dir/old.txt", ItemInfo: iteminfo.ItemInfo{Name: "old.txt", ModTime: time.Unix(50, 0), Type: "text/plain"}, FullTextBody: body("old.txt\nstale")},
	}); err != nil {
		t.Fatal(err)
	}
	// Simulate a row deleted by the stale-item sweep without touching FTS, then the purge pass.
	if _, err := db.Exec(`DELETE FROM index_items WHERE path = '/dir/old.txt'`); err != nil {
		t.Fatal(err)
	}
	if err := db.purgeFullTextOrphans("src", "/dir/"); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"stale"}, 10); len(hits) != 0 {
		t.Fatalf("orphan still matches: %v", hits)
	}
}

func TestFullTextQuerySafety(t *testing.T) {
	db := newFullTextTestDB(t)
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{
		{Path: "/q.txt", ItemInfo: iteminfo.ItemInfo{Name: "q.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("q.txt\nkey: value AND OR NOT \"quoted\"")},
	}); err != nil {
		t.Fatal(err)
	}
	for _, term := range []string{`key:`, `AND`, `OR`, `NOT`, `"quoted"`, "q.txt"} {
		if _, err := db.SearchFullText("src", "/", []string{term}, 10); err != nil {
			t.Fatalf("term %q broke FTS query: %v", term, err)
		}
	}
	if hits, err := db.SearchFullText("src", "/", []string{`key:`}, 10); err != nil || len(hits) != 1 {
		t.Fatalf("colon term hits = %v, err %v", hits, err)
	}
}
