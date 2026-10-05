package web

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestSearchHandlerConfiguredLimit(t *testing.T) {
	original := settings.Config
	originalDB := indexing.GetIndexDB()
	t.Cleanup(func() { settings.Config = original; indexing.SetIndexDBForTesting(originalDB) })
	settings.Config = settings.SetDefaults(true)
	setupTestEnv(t)
	settings.Config.Server.CacheDir = t.TempDir()
	fileutils.SetFsPermissions(0644, 0755)
	db, _, err := dbsql.NewIndexDB("search_limit", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	indexing.SetIndexDBForTesting(db)
	user := &users.User{FrontendUser: users.FrontendUser{Username: "search-user"}}
	for _, name := range []string{"limit-a", "limit-b"} {
		source := &settings.Source{Name: name, Path: filepath.Join(t.TempDir(), name)}
		source.Config.ResolvedRules.IndexingDisabled = true
		settings.Config.Server.NameToSource[name] = source
		settings.Config.Server.SourceMap[source.Path] = source
		user.BackendScopes = append(user.BackendScopes, users.BackendScope{Path: source.Path, Scope: "/"})
		indexing.Initialize(source, true, false)
		files := make([]iteminfo.ExtendedItemInfo, 600)
		for i := range files {
			files[i] = iteminfo.ExtendedItemInfo{ItemInfo: iteminfo.ItemInfo{Name: fmt.Sprintf("document-%03d.txt", i), Type: "text", Size: 1024}}
		}
		indexing.GetIndex(name).UpdateMetadata(&iteminfo.FileInfo{Path: "/", Files: files}, nil, true)
	}
	for _, tc := range []struct {
		name      string
		limit     int
		sources   string
		largest   bool
		requested string
		want      int
	}{
		{"quick default", 1000, "limit-a", false, "", 100},
		{"advanced default", 1000, "limit-a", false, "500", 500},
		{"custom request", 1000, "limit-a", false, "250", 250},
		{"minimum request", 1000, "limit-a", false, "1", 1},
		{"server cap", 250, "limit-a", false, "500", 250},
		{"quick capped", 20, "limit-a", false, "", 20},
		{"combined limit", 1000, "limit-a,limit-b", false, "1000", 1000},
		{"combined capped", 400, "limit-a,limit-b", false, "900", 400},
		{"size viewer unchanged", 10, "limit-a", true, "1", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings.Config.Server.SearchResultsLimit = tc.limit
			req := httptest.NewRequest("GET", fmt.Sprintf("/api/tools/search?query=document&sources=%s&largest=%t&limit=%s", tc.sources, tc.largest, tc.requested), nil)
			recorder := httptest.NewRecorder()
			status, err := searchHandler(recorder, req, &Context{User: user})
			if err != nil {
				t.Fatalf("status %d: %v", status, err)
			}
			var results []indexing.SearchResult
			if err := json.Unmarshal(recorder.Body.Bytes(), &results); err != nil {
				t.Fatal(err)
			}
			if len(results) != tc.want {
				t.Fatalf("want %d results, got %d", tc.want, len(results))
			}
		})
	}

	for _, limit := range []string{"0", "-1", "abc", "1.5", "9999999999999999999999999999"} {
		t.Run("invalid "+limit, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/tools/search?query=document&sources=limit-a&limit="+limit, nil)
			status, err := searchHandler(httptest.NewRecorder(), req, &Context{User: user})
			if status != 400 || err == nil || err.Error() != "limit must be a positive integer" {
				t.Fatalf("expected invalid limit, got status=%d err=%v", status, err)
			}
		})
	}
}
