package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/internal/preview"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// Use a database-backed index because deletion refreshes and removes metadata.
func setupDeleteQuotaTest(t *testing.T) string {
	t.Helper()
	sourcePath := setupPutQuotaTest(t)
	settings.Config.Http.BaseURL = "/"
	fileutils.SetFsPermissions(0644, 0755)
	oldDB := indexing.GetIndexDB()
	settings.Config.Server.CacheDir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(settings.Config.Server.CacheDir, "sql"), 0755); err != nil {
		t.Fatal(err)
	}
	db, _, err := dbsql.NewIndexDB("delete_quota", "OFF", 1000, 32, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		indexing.SetIndexDBForTesting(oldDB)
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	indexing.SetIndexDBForTesting(db)
	source := settings.Config.Server.NameToSource["Downloads"]
	source.Config.UseLogicalSize = true
	source.Config.ResolvedRules.IndexingDisabled = true
	indexing.Initialize(source, true, false)
	indexing.GetIndex("Downloads").Config.ResolvedRules.IndexingDisabled = false
	return sourcePath
}

func TestResourceDeleteNestedQuota(t *testing.T) {
	sourcePath := setupDeleteQuotaTest(t)
	if preview.GetService() == nil {
		if err := preview.StartPreviewGenerator(1, t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	user, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{"public/direct": "123", "public/nested/file": "12345", "sibling/file": "1234567"} {
		full := filepath.Join(sourcePath, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	idx := indexing.GetIndex("Downloads")
	if err := idx.RefreshDirectory("/", true); err != nil {
		t.Fatal(err)
	}
	nested, err := state.CreateFolderQuota(sourcePath, "/public/nested", user.ID, 100, "accounted")
	if err != nil {
		t.Fatal(err)
	}
	sibling, err := state.CreateFolderQuota(sourcePath, "/sibling", user.ID, 100, "accounted")
	if err != nil {
		t.Fatal(err)
	}
	for path, size := range map[string]int64{"/public/nested/file": 5, "/sibling/file": 7} {
		if err := state.ApplyAccountedUsageDelta(&user, "Downloads", sourcePath, path, "", size); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(http.MethodDelete, "/api/resources?source=Downloads&path=/public", nil)
	status, err := resourceDeleteHandler(httptest.NewRecorder(), req, &requestContext{User: &user})
	if err != nil || status != http.StatusOK {
		t.Fatalf("delete: status=%d err=%v", status, err)
	}
	if _, err := os.Stat(filepath.Join(sourcePath, "public")); !os.IsNotExist(err) {
		t.Fatalf("ancestor still exists: %v", err)
	}
	if used, _ := state.GetQuotaCounterSnapshot(nested.ID); used != 0 {
		t.Fatalf("nested usage=%d, want 0", used)
	}
	if used, _ := state.GetQuotaCounterSnapshot(sibling.ID); used != 7 {
		t.Fatalf("sibling usage=%d, want 7", used)
	}
}
