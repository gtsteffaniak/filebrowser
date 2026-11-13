package web

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestSearchHandlerContentSearch(t *testing.T) {
	original := settings.Config
	originalDB := indexing.GetIndexDB()
	t.Cleanup(func() {
		settings.Config = original
		indexing.SetIndexDBForTesting(originalDB)
	})
	settings.Config = settings.SetDefaults(true)
	setupTestEnv(t)
	settings.Config.Server.CacheDir = t.TempDir()
	fileutils.SetFsPermissions(0644, 0755)

	db, _, err := dbsql.NewIndexDB("search_content", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	indexing.SetIndexDBForTesting(db)

	source := &settings.Source{Name: "content-src", Path: filepath.Join(t.TempDir(), "content-src")}
	source.Config.ResolvedRules.IndexingDisabled = true
	settings.Config.Server.NameToSource[source.Name] = source
	settings.Config.Server.SourceMap[source.Path] = source
	user := &users.User{FrontendUser: users.FrontendUser{Username: "content-user"}}
	user.BackendScopes = append(user.BackendScopes, users.BackendScope{Path: source.Path, Scope: "/"})
	indexing.Initialize(source, true, false)
	_ = indexing.GetIndex(source.Name)

	settings.Config.Server.IndexSqlConfig.FullTextSearch = true
	// The search term appears only in a body; rows carry the plain "text" type the
	// scanner stores for common formats, and a realistic modification time.
	body := "notes.txt\nthe rhubarbok plan"
	if err := db.BulkInsertItems(source.Name, []*iteminfo.FileInfo{
		{Path: "/notes.txt", ItemInfo: iteminfo.ItemInfo{Name: "notes.txt", Type: "text", Size: 10, ModTime: time.Now()}, FullTextBody: &body},
	}); err != nil {
		t.Fatal(err)
	}

	run := func(params string) []indexing.SearchResult {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/tools/search?sources="+source.Name+"&"+params, nil)
		recorder := httptest.NewRecorder()
		status, err := searchHandler(recorder, req, &Context{User: user})
		if err != nil {
			t.Fatalf("status %d: %v", status, err)
		}
		var results []indexing.SearchResult
		if err := json.Unmarshal(recorder.Body.Bytes(), &results); err != nil {
			t.Fatal(err)
		}
		return results
	}

	got := run("query=rhubarbok&searchContent=true")
	if len(got) != 1 || got[0].Path != "notes.txt" || got[0].Type == "directory" {
		t.Fatalf("content search results = %#v", got)
	}
	if got := run("query=rhubarbok"); len(got) != 0 {
		t.Fatalf("content matched without searchContent: %#v", got)
	}
	settings.Config.Server.IndexSqlConfig.FullTextSearch = false
	req := httptest.NewRequest("GET", "/api/tools/search?sources="+source.Name+"&query=rhubarbok&searchContent=true", nil)
	if status, err := searchHandler(httptest.NewRecorder(), req, &Context{User: user}); err == nil || status != 400 {
		t.Fatalf("want 400 when full-text search is disabled, got %d %v", status, err)
	}
}
