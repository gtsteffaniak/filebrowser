package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupPutQuotaTest(t *testing.T) string {
	t.Helper()
	downloadsPath := filepath.Join(t.TempDir(), "downloads")

	dbPath := filepath.Join(t.TempDir(), "quota-http-test.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() {
		state.Close()
	})

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		downloadsPath: {
			Path: downloadsPath,
			Name: "Downloads",
		},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"Downloads": settings.Config.Server.SourceMap[downloadsPath],
	}
	settings.InitializeUserResolvers()

	adminUser := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "admin",
			Permissions: users.Permissions{Admin: true},
		},
		BackendScopes: []users.BackendScope{
			{Path: downloadsPath, Scope: "/"},
		},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			downloadsPath: {
				View:     true,
				Download: true,
				Modify:   true,
				Create:   true,
				Delete:   true,
			},
		},
		Version: users.SourcePermissionsMigrationVersion,
	}
	if err := state.CreateUser(adminUser, ""); err != nil {
		t.Fatal(err)
	}
	adminUser.Permissions = users.Permissions{Admin: true}
	if err := state.UpdateUser(adminUser, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	if err := state.UpdateUser(adminUser, "", "backendScopes"); err != nil {
		t.Fatal(err)
	}
	if err := state.UpdateUser(adminUser, "", "backendSourcePermissions"); err != nil {
		t.Fatal(err)
	}

	indexing.SetTestIndex("Downloads", downloadsPath)
	t.Cleanup(func() {
		indexing.ClearTestIndices()
	})

	admin, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := state.CreateFolderQuota(downloadsPath, "/projects", admin.ID, 5*1024*1024, "accounted"); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(downloadsPath, "projects", "edit.txt")
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatal(err)
	}
	oneMB := strings.Repeat("a", 1024*1024)
	if err := os.WriteFile(filePath, []byte(oneMB), 0644); err != nil {
		t.Fatal(err)
	}

	return downloadsPath
}

func TestResourcePutHandler_RejectsGrowthOverQuota(t *testing.T) {
	downloadsPath := setupPutQuotaTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	sevenMB := strings.Repeat("b", 7*1024*1024)
	req := httptest.NewRequest(http.MethodPut, "/api/resources?source=Downloads&path=/projects/edit.txt", strings.NewReader(sevenMB))
	req.ContentLength = int64(len(sevenMB))
	rec := httptest.NewRecorder()

	status, handlerErr := resourcePutHandler(rec, req, ctx)
	if status != http.StatusInsufficientStorage {
		t.Fatalf("expected 507, got status=%d err=%v body=%s", status, handlerErr, rec.Body.String())
	}

	onDisk, err := os.ReadFile(filepath.Join(downloadsPath, "projects", "edit.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(onDisk) != 1024*1024 {
		t.Fatalf("file size changed after rejected put: got %d want %d", len(onDisk), 1024*1024)
	}
}

func TestResourcePutHandler_AllowsGrowthWithinQuota(t *testing.T) {
	setupPutQuotaTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	twoMB := strings.Repeat("c", 2*1024*1024)
	req := httptest.NewRequest(http.MethodPut, "/api/resources?source=Downloads&path=/projects/edit.txt", strings.NewReader(twoMB))
	req.ContentLength = int64(len(twoMB))
	rec := httptest.NewRecorder()

	status, handlerErr := resourcePutHandler(rec, req, ctx)
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("put failed: status=%d err=%v body=%s", status, handlerErr, rec.Body.String())
	}
}

func TestWebDAVPut_RejectsGrowthOverQuota(t *testing.T) {
	downloadsPath := setupPutQuotaTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}

	settings.Config.Http.BaseURL = "/"

	sevenMB := strings.Repeat("d", 7*1024*1024)
	req := httptest.NewRequest(http.MethodPut, "/dav/Downloads/projects/edit.txt", strings.NewReader(sevenMB))
	req.ContentLength = int64(len(sevenMB))
	req.SetPathValue("source", "Downloads")
	req.SetPathValue("path", "/projects/edit.txt")

	rec := httptest.NewRecorder()
	status, handlerErr := webDAVHandler(rec, req, &requestContext{User: &adminUser})
	if handlerErr != nil {
		t.Fatalf("webdav handler error: %v", handlerErr)
	}
	if status == http.StatusOK && rec.Code == http.StatusCreated {
		t.Fatalf("expected put to fail, got status=%d code=%d", status, rec.Code)
	}

	onDisk, err := os.ReadFile(filepath.Join(downloadsPath, "projects", "edit.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(onDisk) != 1024*1024 {
		t.Fatalf("file size changed after rejected webdav put: got %d want %d", len(onDisk), 1024*1024)
	}
}
