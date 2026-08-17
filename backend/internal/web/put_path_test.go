package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	commonerrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func symlinkPutFixture(t *testing.T) (downloadsPath string, idx *indexing.Index) {
	t.Helper()
	downloadsPath = filepath.Join(t.TempDir(), "downloads")
	if err := os.MkdirAll(filepath.Join(downloadsPath, "projects"), 0755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(downloadsPath), "outside-target")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(downloadsPath, "evil")); err != nil {
		t.Fatal(err)
	}
	idx = &indexing.Index{Source: settings.Source{Path: downloadsPath}}
	return downloadsPath, idx
}

func TestResolvePutRealPath_NewFileViaParent(t *testing.T) {
	_, idx := symlinkPutFixture(t)

	realPath, err := resolvePutRealPath(idx, "/projects/newfile.txt")
	if err != nil {
		t.Fatalf("resolvePutRealPath: %v", err)
	}
	parentReal, isDir, err := idx.GetRealPath("/projects")
	if err != nil || !isDir {
		t.Fatalf("parent dir: err=%v isDir=%v", err, isDir)
	}
	want := filepath.Join(parentReal, "newfile.txt")
	if realPath != want {
		t.Fatalf("got %q want %q", realPath, want)
	}
}

func TestResolvePutRealPath_RejectsEscapingParent(t *testing.T) {
	_, idx := symlinkPutFixture(t)

	_, err := resolvePutRealPath(idx, "/evil/hack.txt")
	if !errors.Is(err, commonerrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope, got %v", err)
	}
}

func TestSanitizePutPathError_NoFilesystemPaths(t *testing.T) {
	_, idx := symlinkPutFixture(t)
	_, internalErr := resolvePutRealPath(idx, "/evil/hack.txt")
	if internalErr == nil {
		t.Fatal("expected resolution error")
	}

	status, clientErr := sanitizePutPathError(internalErr)
	if status != http.StatusForbidden {
		t.Fatalf("status=%d want 403", status)
	}
	if clientErr.Error() != "access denied" {
		t.Fatalf("message=%q want access denied", clientErr.Error())
	}
	if strings.Contains(clientErr.Error(), "outside") || strings.Contains(clientErr.Error(), "evil") {
		t.Fatalf("client error leaked path details: %q", clientErr.Error())
	}
}

func TestResourcePutHandler_CreatesNewFile(t *testing.T) {
	downloadsPath := setupPutHTTPTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := &requestContext{User: &adminUser}

	body := "hello new file"
	req := httptest.NewRequest(http.MethodPut, "/api/resources?source=Downloads&path=/projects/newfile.txt", strings.NewReader(body))
	req.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()

	status, handlerErr := resourcePutHandler(rec, req, ctx)
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("put failed: status=%d err=%v", status, handlerErr)
	}

	destPath := filepath.Join(downloadsPath, "projects", "newfile.txt")
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len(body)) {
		t.Fatalf("size=%d want %d", info.Size(), len(body))
	}
}

func TestResourcePutHandler_RejectsEscapingParentSymlink(t *testing.T) {
	downloadsPath := setupPutHTTPTest(t)
	outside := filepath.Join(filepath.Dir(downloadsPath), "outside-target")
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(downloadsPath, "evil")); err != nil {
		t.Fatal(err)
	}

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}

	body := "should not land outside"
	req := httptest.NewRequest(http.MethodPut, "/api/resources?source=Downloads&path=/evil/hack.txt", strings.NewReader(body))
	req.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()

	status, handlerErr := resourcePutHandler(rec, req, &requestContext{User: &adminUser})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v", status, handlerErr)
	}
	if handlerErr == nil || handlerErr.Error() != "access denied" {
		t.Fatalf("err=%v want access denied", handlerErr)
	}
	if strings.Contains(handlerErr.Error(), downloadsPath) || strings.Contains(handlerErr.Error(), outside) {
		t.Fatalf("error leaked path: %q", handlerErr.Error())
	}
}

func TestResourcePutHandler_SuccessResponseOmitsRealPath(t *testing.T) {
	downloadsPath := setupPutHTTPTest(t)

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}

	body := "ok"
	req := httptest.NewRequest(http.MethodPut, "/api/resources?source=Downloads&path=/projects/another.txt", strings.NewReader(body))
	req.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()

	status, handlerErr := resourcePutHandler(rec, req, &requestContext{User: &adminUser})
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("status=%d err=%v body=%s", status, handlerErr, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), downloadsPath) {
		t.Fatalf("response leaked filesystem path: %s", rec.Body.String())
	}
}

func setupPutHTTPTest(t *testing.T) string {
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

	if err := os.MkdirAll(filepath.Join(downloadsPath, "projects"), 0755); err != nil {
		t.Fatal(err)
	}

	return downloadsPath
}
