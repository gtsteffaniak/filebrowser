package web

import (
	"archive/zip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/webdav"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setTestSourceReadOnly(sourceName string, readOnly bool) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return
	}
	idx.Config.ReadOnly = readOnly
}

func TestDenyIfSourceReadOnly(t *testing.T) {
	t.Parallel()
	indexing.SetTestIndex("ro-test", t.TempDir())
	t.Cleanup(func() { indexing.ClearTestIndices() })

	if status, err := denyIfSourceReadOnly("ro-test"); status != 0 || err != nil {
		t.Fatalf("writable source: status=%d err=%v", status, err)
	}

	setTestSourceReadOnly("ro-test", true)
	status, err := denyIfSourceReadOnly("ro-test")
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("read-only source: status=%d err=%v", status, err)
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("error = %v", err)
	}
}

func TestUnarchivePreviewAllowsReadOnlyDestination(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	setTestSourceReadOnly("uploads", true)

	archivePath := filepath.Join(root, "test.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	body := `{"fromSource":"uploads","path":"/test.zip","destination":"/","preview":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/unarchive", strings.NewReader(body))
	status, err := unarchiveHandler(rec, req, &Context{User: user})
	if err != nil || status != http.StatusOK {
		t.Fatalf("preview on read-only dest: status=%d err=%v body=%s", status, err, rec.Body.String())
	}
}

func TestUnarchiveExtractionDeniedOnReadOnlyDestination(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	setTestSourceReadOnly("uploads", true)

	archivePath := filepath.Join(root, "test.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	body := `{"fromSource":"uploads","path":"/test.zip","destination":"/","preview":false}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/unarchive", strings.NewReader(body))
	status, err := unarchiveHandler(rec, req, &Context{User: user})
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("extract on read-only dest: status=%d err=%v", status, err)
	}
}

func TestWebDAVMutationsDeniedOnReadOnlySource(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	setTestSourceReadOnly("source1", true)

	user := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{Username: "writer"},
		BackendScopes: []users.BackendScope{
			{Path: source1Path, Scope: "/"},
		},
		BackendSourcePermissions: webDAVPermsForPaths(true, true, true, true, source1Path),
		Version:                  users.SourcePermissionsMigrationVersion,
	}
	applyBackendSourcePerms(user, user.BackendSourcePermissions)

	ffs := &filteredFileSystem{
		fs:        webdav.Dir(filepath.Join(source1Path, "public")),
		source:    "source1",
		user:      user,
		filePerms: user.BackendSourcePermissions[source1Path],
	}

	ctx := context.Background()
	if err := ffs.Mkdir(ctx, "rodir", 0o755); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("Mkdir: err=%v", err)
	}
	if err := ffs.RemoveAll(ctx, "readme.txt"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("RemoveAll: err=%v", err)
	}
	if err := ffs.Rename(ctx, "readme.txt", "moved.txt"); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("Rename: err=%v", err)
	}
	_, err := ffs.OpenFile(ctx, "new.txt", os.O_CREATE|os.O_WRONLY, 0o644)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("OpenFile write: err=%v", err)
	}
}

func TestOnlyOfficeCallbackDeniedOnReadOnlySource(t *testing.T) {
	initStreamTestSources(t)
	indexing.SetTestIndex("default", "/default")
	t.Cleanup(func() { indexing.ClearTestIndices() })
	setTestSourceReadOnly("default", true)

	const realPath = "/default/doc.docx"
	const docKey = "oo-ro-key"
	origFileInfo := files.FileInfoFasterFunc
	t.Cleanup(func() { files.FileInfoFasterFunc = origFileInfo })
	files.FileInfoFasterFunc = func(utils.FileOptions, *users.User) (*iteminfo.ExtendedFileInfo, error) {
		return &iteminfo.ExtendedFileInfo{RealPath: realPath}, nil
	}
	utils.OnlyOfficeCache.Set(realPath, docKey)
	t.Cleanup(func() { utils.OnlyOfficeCache.Delete(realPath) })

	origOnlyOffice := settings.Config.Integrations.OnlyOffice
	t.Cleanup(func() { settings.Config.Integrations.OnlyOffice = origOnlyOffice })
	settings.Config.Integrations.OnlyOffice.Url = "http://onlyoffice.test"

	user := testUserWithSourcePerms("/default", users.SourceFilePermissions{
		View: true, Download: true, Modify: true,
	})
	d := &requestContext{User: user}
	callback := &OnlyOfficeCallback{
		Key:    docKey,
		Status: onlyOfficeStatusDocumentClosedWithChanges,
		URL:    "http://onlyoffice.test/cache/doc.docx",
	}
	req := httptest.NewRequest(http.MethodPost, "/api/office/callback?source=default&path=/doc.docx", nil)
	rec := httptest.NewRecorder()
	if _, err := processOnlyOfficeCallback(rec, req, d, callback); err != nil {
		t.Fatalf("processOnlyOfficeCallback: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestUnarchivePreviewSkipsReadOnlyDeleteAfterOnSource(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	setTestSourceReadOnly("uploads", true)

	archivePath := filepath.Join(root, "test.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	body := `{"fromSource":"uploads","path":"/test.zip","destination":"/","preview":true,"deleteAfter":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/unarchive", strings.NewReader(body))
	status, err := unarchiveHandler(rec, req, &Context{User: user})
	if err != nil || status != http.StatusOK {
		t.Fatalf("preview with deleteAfter on read-only source: status=%d err=%v", status, err)
	}
	var preview json.RawMessage
	if err = json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(archivePath); err != nil {
		t.Fatal("archive should remain when preview does not delete")
	}
}
