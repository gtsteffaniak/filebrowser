package indexing

import (
	"strings"
	"testing"
	"time"

	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func newFullTextSearchTestIndex(t *testing.T, name string) *Index {
	t.Helper()
	if indexDB == nil {
		var err error
		indexDB, _, err = dbsql.NewIndexDB("test_search_fulltext", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}
	idx := &Index{
		Source: settings.Source{Name: name, Path: "/mock/path"},
		db:     indexDB,
		mock:   true,
	}
	insert := func(path string, info iteminfo.ItemInfo, body string) {
		info.ModTime = time.Unix(1700000000, 0)
		file := &iteminfo.FileInfo{Path: path, ItemInfo: info}
		if body != "" {
			file.FullTextBody = &body
		}
		if err := indexDB.BulkInsertItems(name, []*iteminfo.FileInfo{file}); err != nil {
			t.Fatal(err)
		}
	}

	insert("/docs/budget.txt", iteminfo.ItemInfo{Name: "budget.txt", Type: "text/plain", Size: 10}, "budget.txt\nquarterly numbers")
	insert("/docs/other.txt", iteminfo.ItemInfo{Name: "other.txt", Type: "text/plain", Size: 10}, "other.txt\nunrelated")
	insert("/docs/report.txt", iteminfo.ItemInfo{Name: "report.txt", Type: "text/plain", Size: 10}, "report.txt\nthe budget summary")
	return idx
}

func TestSearchMatchesFileContent(t *testing.T) {
	idx := newFullTextSearchTestIndex(t, "fulltext_content")
	results := idx.SearchParsed(iteminfo.SearchOptions{Terms: []string{"budget"}}, "/", "", false, 100, 0, 0, false, true)
	paths := make(map[string]bool, len(results))
	for _, r := range results {
		paths[r.Path] = true
	}
	if !paths["/docs/budget.txt"] || !paths["/docs/report.txt"] {
		t.Fatalf("expected name and content hits, got %#v", results)
	}
	if paths["/docs/other.txt"] {
		t.Fatalf("unrelated file matched: %#v", results)
	}
}

func TestSearchContentRespectsScopeAndLimit(t *testing.T) {
	idx := newFullTextSearchTestIndex(t, "fulltext_scope")
	scoped := idx.SearchParsed(iteminfo.SearchOptions{Terms: []string{"budget"}}, "/nonexistent/", "", false, 100, 0, 0, false, true)
	if len(scoped) != 0 {
		t.Fatalf("scope not applied to content hits: %#v", scoped)
	}
	limited := idx.SearchParsed(iteminfo.SearchOptions{Terms: []string{"budget"}}, "/", "", false, 1, 0, 0, false, true)
	if len(limited) != 1 {
		t.Fatalf("limit not applied: %#v", limited)
	}
	// Wildcard search stays name-only.
	wild := idx.SearchParsed(iteminfo.SearchOptions{Terms: []string{"bud*"}}, "/", "", false, 100, 0, 0, true, true)
	for _, r := range wild {
		if r.Path == "/docs/report.txt" {
			t.Fatalf("wildcard search must not match content: %#v", wild)
		}
	}
}

func TestSearchContentMultiSource(t *testing.T) {
	a := newFullTextSearchTestIndex(t, "fulltext_multi_a")
	b := newFullTextSearchTestIndex(t, "fulltext_multi_b")
	_ = a
	_ = b
	results := SearchMultiSourcesParsed(iteminfo.SearchOptions{Terms: []string{"quarterly"}}, []string{"fulltext_multi_a", "fulltext_multi_b"}, map[string]string{
		"fulltext_multi_a": "/",
		"fulltext_multi_b": "/",
	}, "", false, 100, 0, 0, false, true)
	if len(results) != 2 {
		t.Fatalf("want one content hit per source, got %#v", results)
	}
	for _, r := range results {
		if !strings.HasSuffix(r.Path, "/docs/budget.txt") {
			t.Fatalf("unexpected multi-source result %#v", r)
		}
	}
}

func TestSearchContentHonorsAuxiliaryFiltersAndJoinMode(t *testing.T) {
	idx := newFullTextSearchTestIndex(t, "fulltext_filters")
	// Folders-only must not return content hits from files.
	foldersOnly := iteminfo.SearchOptions{Conditions: map[string]bool{"dir": true}, Terms: []string{"budget"}}
	for _, r := range idx.SearchParsed(foldersOnly, "/", "", false, 100, 0, 0, false, true) {
		if r.Type != "directory" {
			t.Fatalf("folders-only search returned a file %#v", r)
		}
	}
	// OR mode matches a content-only file on any single term.
	orHits := idx.SearchParsed(iteminfo.SearchOptions{Terms: []string{"quarterly", "nonexistentterm"}}, "/", "", false, 100, 0, 0, false, true)
	found := false
	for _, r := range orHits {
		if r.Path == "/docs/budget.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("OR mode missed content-only match: %#v", orHits)
	}
	// AND mode requires every term.
	andHits := idx.SearchParsed(iteminfo.SearchOptions{Terms: []string{"quarterly", "nonexistentterm"}, MatchAllTerms: true}, "/", "", false, 100, 0, 0, false, true)
	for _, r := range andHits {
		if r.Path == "/docs/budget.txt" {
			t.Fatalf("AND mode returned a partial content match: %#v", andHits)
		}
	}
}
