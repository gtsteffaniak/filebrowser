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

	paths, err := db.SearchFullText("src", "/", []string{"budget"}, true, 100, "")
	if err != nil {
		t.Fatalf("SearchFullText: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("want 2 content hits, got %v", paths)
	}

	// Scope restricts to the given folder prefix.
	scoped, err := db.SearchFullText("src", "/notes/", []string{"budget"}, true, 100, "")
	if err != nil {
		t.Fatalf("SearchFullText scoped: %v", err)
	}
	if len(scoped) != 1 || scoped[0] != "/notes/annual report.md" {
		t.Fatalf("scoped hits = %v", scoped)
	}

	// Every term must match (AND).
	both, err := db.SearchFullText("src", "/", []string{"budget", "quarter"}, true, 100, "")
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
	if hits, _ := db.SearchFullText("src", "/", []string{"oldword"}, true, 10, ""); len(hits) != 0 {
		t.Fatalf("stale body still matches: %v", hits)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"newword"}, true, 10, ""); len(hits) != 1 {
		t.Fatalf("updated body not searchable: %v", hits)
	}

	// Updating without a body clears the content row (name still searchable).
	item.FullTextBody = nil
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{item}); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"newword"}, true, 10, ""); len(hits) != 0 {
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
	if _, err := db.PurgeFullTextOrphans("src", "/dir/"); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"stale"}, true, 10, ""); len(hits) != 0 {
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
		if _, err := db.SearchFullText("src", "/", []string{term}, true, 10, ""); err != nil {
			t.Fatalf("term %q broke FTS query: %v", term, err)
		}
	}
	if hits, err := db.SearchFullText("src", "/", []string{`key:`}, true, 10, ""); err != nil || len(hits) != 1 {
		t.Fatalf("colon term hits = %v, err %v", hits, err)
	}
}

func TestFullTextUpsertAfterConflictKeepsEachRow(t *testing.T) {
	db := newFullTextTestDB(t)
	a := &iteminfo.FileInfo{Path: "/a.txt", ItemInfo: iteminfo.ItemInfo{Name: "a.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("a.txt\nalpha")}
	b := &iteminfo.FileInfo{Path: "/b.txt", ItemInfo: iteminfo.ItemInfo{Name: "b.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("b.txt\nbeta")}
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{a}); err != nil {
		t.Fatal(err)
	}
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{b}); err != nil {
		t.Fatal(err)
	}
	// Updating the first file hits ON CONFLICT DO UPDATE; B's row must survive.
	a.FullTextBody = body("a.txt\nalpha updated")
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{a}); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"beta"}, true, 10, ""); len(hits) != 1 || hits[0] != "/b.txt" {
		t.Fatalf("b.txt content lost or misplaced: %v", hits)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"updated"}, true, 10, ""); len(hits) != 1 || hits[0] != "/a.txt" {
		t.Fatalf("a.txt content not updated: %v", hits)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"alpha"}, true, 10, ""); len(hits) != 1 {
		t.Fatalf("stale a.txt body still matches: %v", hits)
	}
}

func TestFullTextLimitKeepsShallowest(t *testing.T) {
	db := newFullTextTestDB(t)
	items := []*iteminfo.FileInfo{
		{Path: "/top.txt", ItemInfo: iteminfo.ItemInfo{Name: "top.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("top.txt\nneedle")},
		{Path: "/deep/a/very/longer/path.txt", ItemInfo: iteminfo.ItemInfo{Name: "path.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("path.txt\nneedle")},
		{Path: "/mid/level.txt", ItemInfo: iteminfo.ItemInfo{Name: "level.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("level.txt\nneedle")},
	}
	if err := db.BulkInsertItems("src", items); err != nil {
		t.Fatal(err)
	}
	hits, err := db.SearchFullText("src", "/", []string{"needle"}, true, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0] != "/top.txt" {
		t.Fatalf("limited hits should keep the shallowest match, got %v", hits)
	}
	paged, err := db.SearchFullText("src", "/", []string{"needle"}, true, 10, "/top.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(paged) != 2 {
		t.Fatalf("paging past /top.txt should keep the two deeper hits, got %v", paged)
	}
}

func TestFullTextJoinMode(t *testing.T) {
	db := newFullTextTestDB(t)
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{
		{Path: "/alpha.txt", ItemInfo: iteminfo.ItemInfo{Name: "alpha.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("alpha.txt\nalpha only")},
	}); err != nil {
		t.Fatal(err)
	}
	terms := []string{"alpha", "beta"}
	if hits, err := db.SearchFullText("src", "/", terms, false, 10, ""); err != nil || len(hits) != 1 {
		t.Fatalf("OR mode should match single-term content: %v %v", hits, err)
	}
	if hits, err := db.SearchFullText("src", "/", terms, true, 10, ""); err != nil || len(hits) != 0 {
		t.Fatalf("AND mode should require every term: %v %v", hits, err)
	}
}

func TestDeleteItemKeepsFullTextInSync(t *testing.T) {
	db := newFullTextTestDB(t)
	if err := db.BulkInsertItems("src", []*iteminfo.FileInfo{
		{Path: "/dir/keep.txt", ItemInfo: iteminfo.ItemInfo{Name: "keep.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("keep.txt\nkeepword")},
		{Path: "/dir/drop.txt", ItemInfo: iteminfo.ItemInfo{Name: "drop.txt", ModTime: time.Unix(100, 0), Type: "text/plain"}, FullTextBody: body("drop.txt\ndropword")},
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteItem("src", "/dir/drop.txt", false); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"dropword"}, true, 10, ""); len(hits) != 0 {
		t.Fatalf("deleted file still searchable: %v", hits)
	}
	if err := db.DeleteItem("src", "/dir/", true); err != nil {
		t.Fatal(err)
	}
	if hits, _ := db.SearchFullText("src", "/", []string{"keepword"}, true, 10, ""); len(hits) != 0 {
		t.Fatalf("recursive delete left content: %v", hits)
	}
}
