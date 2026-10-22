package sql

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func newSearchTestDB(t *testing.T, name string) *IndexDB {
	t.Helper()
	dir := t.TempDir()
	pop := pushTestIndexConfig(t, dir, testIndexSQLConfig(settings.IndexStartupIntegrityOff))
	t.Cleanup(pop)
	db, _, err := NewIndexDB(name, "OFF", 1000, 32, false)
	if err != nil {
		t.Fatalf("NewIndexDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func insertSearchItem(t *testing.T, db *IndexDB, source, path, name string) {
	t.Helper()
	info := &iteminfo.FileInfo{
		Path: path,
		ItemInfo: iteminfo.ItemInfo{
			Name:    name,
			Type:    "text/plain",
			ModTime: time.Unix(1, 0),
		},
	}
	if err := db.InsertItem(source, path, info); err != nil {
		t.Fatalf("InsertItem(%s): %v", path, err)
	}
}

func runSearchItems(t *testing.T, db *IndexDB, source, scope string, filter *iteminfo.NameFilter) []string {
	t.Helper()
	rows, err := db.SearchItems(context.Background(), source, scope, false, filter)
	if err != nil {
		t.Fatalf("SearchItems: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var path, name string
		var size, createdTime, modTime int64
		var mimeType string
		var isDir, hasPreview bool
		if err := rows.Scan(&path, &name, &size, &createdTime, &modTime, &mimeType, &isDir, &hasPreview); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, path)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return got
}

func seedSearchItems(t *testing.T, db *IndexDB) {
	t.Helper()
	insertSearchItem(t, db, "s1", "/a/Report 2024.pdf", "Report 2024.pdf")
	insertSearchItem(t, db, "s1", "/a/my_file.txt", "my_file.txt")
	insertSearchItem(t, db, "s1", "/a/myXfile.txt", "myXfile.txt")
	insertSearchItem(t, db, "s1", "/a/abc.txt", "abc.txt")
	insertSearchItem(t, db, "s1", "/b/report.txt", "report.txt")
	insertSearchItem(t, db, "s2", "/a/report-other.txt", "report-other.txt")
}

func TestSearchItemsFTSDefault(t *testing.T) {
	db := newSearchTestDB(t, "search_fts_default")
	if !db.FTSReady() {
		t.Fatal("expected the FTS index to be ready")
	}
	seedSearchItems(t, db)

	filter := iteminfo.NameFilterForSearch(iteminfo.SearchOptions{Terms: []string{"report"}}, false, false)
	if filter == nil || !filter.UseFTS {
		t.Fatalf("expected an FTS filter, got %#v", filter)
	}
	got := runSearchItems(t, db, "s1", "/", filter)
	want := []string{"/a/Report 2024.pdf", "/b/report.txt"}
	if !sameSet(got, want) {
		t.Fatalf("default search got %v, want %v", got, want)
	}

	// The LIKE prefilter treats '_' as a wildcard; the exact lower(name) GLOB must remove
	// the false positive myXfile.txt.
	filter = iteminfo.NameFilterForSearch(iteminfo.SearchOptions{Terms: []string{"my_file"}}, false, false)
	got = runSearchItems(t, db, "s1", "/", filter)
	want = []string{"/a/my_file.txt"}
	if !sameSet(got, want) {
		t.Fatalf("underscore search got %v, want %v", got, want)
	}

	// Scope is respected.
	filter = iteminfo.NameFilterForSearch(iteminfo.SearchOptions{Terms: []string{"report"}}, false, false)
	got = runSearchItems(t, db, "s1", "/b/", filter)
	want = []string{"/b/report.txt"}
	if !sameSet(got, want) {
		t.Fatalf("scoped search got %v, want %v", got, want)
	}
}

func TestSearchItemsCaseExactAndWildcard(t *testing.T) {
	db := newSearchTestDB(t, "search_fts_modes")
	seedSearchItems(t, db)

	// case:exact is case-sensitive.
	exact := iteminfo.NameFilterForSearch(iteminfo.SearchOptions{
		Terms:      []string{"Report"},
		Conditions: map[string]bool{"exact": true},
	}, false, false)
	got := runSearchItems(t, db, "s1", "/", exact)
	want := []string{"/a/Report 2024.pdf"}
	if !sameSet(got, want) {
		t.Fatalf("case:exact got %v, want %v", got, want)
	}

	// Wildcard stays on the legacy (case-insensitive) path.
	wild := iteminfo.NameFilterForSearch(iteminfo.SearchOptions{Terms: []string{"report*"}}, true, false)
	if wild == nil || wild.UseFTS {
		t.Fatalf("wildcard should use the legacy path, got %#v", wild)
	}
	got = runSearchItems(t, db, "s1", "/", wild)
	want = []string{"/a/Report 2024.pdf", "/b/report.txt"}
	if !sameSet(got, want) {
		t.Fatalf("wildcard got %v, want %v", got, want)
	}

	// Too short for trigrams => legacy path, still correct.
	short := iteminfo.NameFilterForSearch(iteminfo.SearchOptions{Terms: []string{"ab"}}, false, false)
	if short == nil || short.UseFTS {
		t.Fatalf("short term should use the legacy path, got %#v", short)
	}
	got = runSearchItems(t, db, "s1", "/", short)
	want = []string{"/a/abc.txt"}
	if !sameSet(got, want) {
		t.Fatalf("short search got %v, want %v", got, want)
	}
}

func TestSearchItemsMultiSource(t *testing.T) {
	db := newSearchTestDB(t, "search_fts_multi")
	seedSearchItems(t, db)

	filter := iteminfo.NameFilterForSearch(iteminfo.SearchOptions{Terms: []string{"report"}}, false, false)
	rows, err := db.SearchItemsMultiSource(context.Background(),
		[]string{"s1", "s2"}, map[string]string{"s1": "/", "s2": "/"}, false, filter)
	if err != nil {
		t.Fatalf("SearchItemsMultiSource: %v", err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var source, path, name string
		var size, ct, mt int64
		var mime string
		var isDir, hasPreview bool
		if err := rows.Scan(&source, &path, &name, &size, &ct, &mt, &mime, &isDir, &hasPreview); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, source+":"+path)
	}
	want := []string{"s1:/a/Report 2024.pdf", "s1:/b/report.txt", "s2:/a/report-other.txt"}
	if !sameSet(got, want) {
		t.Fatalf("multi-source got %v, want %v", got, want)
	}
}

// TestSearchItemsQueryPlan guards the planner fix: the FTS table must drive the query.
func TestSearchItemsQueryPlan(t *testing.T) {
	db := newSearchTestDB(t, "search_fts_plan")
	seedSearchItems(t, db)

	plan, err := db.ExplainQueryPlan(
		`SELECT i.path FROM index_items_fts f CROSS JOIN index_items i ON i.rowid = f.rowid
		 WHERE i.source = ? AND i.path GLOB ? AND f.spath LIKE ? AND f.name LIKE ? AND lower(i.name) GLOB ?`,
		"s1", "/%", "s1:/%", "%report%", "*report*",
	)
	if err != nil {
		t.Fatalf("ExplainQueryPlan: %v", err)
	}
	if !strings.Contains(plan, "VIRTUAL TABLE") {
		t.Fatalf("expected the FTS table to drive the query, plan:\n%s", plan)
	}
	if !strings.Contains(plan, "SEARCH i USING INTEGER PRIMARY KEY") {
		t.Fatalf("expected a rowid lookup into index_items, plan:\n%s", plan)
	}
}

func TestSearchItemsFTSHealthAfterDelete(t *testing.T) {
	db := newSearchTestDB(t, "search_fts_health")
	seedSearchItems(t, db)
	if err := db.checkFTSHealth(); err != nil {
		t.Fatalf("health before delete: %v", err)
	}
	if err := db.DeleteItem("s1", "/a/abc.txt", false); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	if err := db.checkFTSHealth(); err != nil {
		t.Fatalf("health after delete: %v", err)
	}
	// A name-changing upsert must also keep the index in sync.
	insertSearchItem(t, db, "s1", "/a/abc.txt", "renamed.txt")
	if err := db.checkFTSHealth(); err != nil {
		t.Fatalf("health after re-insert: %v", err)
	}
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	set := make(map[string]int, len(got))
	for _, g := range got {
		set[g]++
	}
	for _, w := range want {
		set[w]--
		if set[w] < 0 {
			return false
		}
	}
	return true
}
