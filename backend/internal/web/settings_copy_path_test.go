package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestSourceInfoCopyPathAccess(t *testing.T) {
	original := settings.Config
	originalDB := indexing.GetIndexDB()
	originalResolver := users.GetSourceConfig()
	t.Cleanup(func() {
		settings.Config = original
		indexing.SetIndexDBForTesting(originalDB)
		users.SetSourceConfig(originalResolver)
	})
	settings.Config = settings.SetDefaults(true)
	setupTestEnv(t)
	settings.Config.Server.CacheDir = t.TempDir()
	db, _, err := dbsql.NewIndexDB("copy_path_access", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	indexing.SetIndexDBForTesting(db)
	user := &users.User{FrontendUser: users.FrontendUser{Username: "copy-path-user"}}
	settings.Config.Server.Sources = nil
	for _, source := range []*settings.Source{
		{Name: "copy-home", Path: "/private/host/home", Config: settings.SourceConfig{CopyPathPrefix: "/home/rt"}},
		{Name: "copy-rt", Path: "/private/host/rt", Config: settings.SourceConfig{CopyPathWindows: true}},
		{Name: "copy-legacy", Path: "/private/host/legacy"},
		{Name: "copy-secret", Path: "/private/host/secret", Config: settings.SourceConfig{CopyPathPrefix: "/secret"}},
	} {
		source.Config.ResolvedRules.IndexingDisabled = true
		settings.Config.Server.Sources = append(settings.Config.Server.Sources, source)
		settings.Config.Server.NameToSource[source.Name] = source
		settings.Config.Server.SourceMap[source.Path] = source
		indexing.Initialize(source, true, false)
		cacheKey := "usageCache-" + source.Name
		utils.DiskUsageCache.Set(cacheKey, true)
		t.Cleanup(func() { utils.DiskUsageCache.Delete(cacheKey) })
		if source.Name != "copy-secret" {
			user.BackendScopes = append(user.BackendScopes, users.BackendScope{Path: source.Path, Scope: "/"})
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/settings/sources", http.NoBody)
	status, err := getSourceInfoHandler(recorder, request, &Context{User: user})
	if err != nil || status != http.StatusOK {
		t.Fatalf("status=%d error=%v", status, err)
	}
	var response map[string]map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response) != 3 || response["copy-secret"] != nil {
		t.Fatalf("source info must be limited to accessible sources: %s", recorder.Body.String())
	}
	if string(response["copy-home"]["copyPathPrefix"]) != `"/home/rt"` || string(response["copy-rt"]["copyPathWindows"]) != "true" {
		t.Fatalf("configured clipboard mappings missing: %s", recorder.Body.String())
	}
	if response["copy-legacy"]["copyPathPrefix"] != nil || response["copy-legacy"]["copyPathWindows"] != nil {
		t.Fatalf("legacy source must omit clipboard mappings: %s", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "/private/host") || strings.Contains(recorder.Body.String(), "/secret") {
		t.Fatalf("host or inaccessible clipboard path leaked: %s", recorder.Body.String())
	}
}
