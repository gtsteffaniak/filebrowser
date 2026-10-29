package web

import (
	"archive/zip"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func webDAVReadOnlyTestUser(source1Path string) *users.User {
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
	return user
}

func callWebDAVHandler(t *testing.T, user *users.User, method, path string, body io.Reader, extra func(*http.Request)) (int, int) {
	t.Helper()
	req := httptest.NewRequest(method, "/dav/source1"+path, body)
	req.SetPathValue("source", "source1")
	req.SetPathValue("path", path)
	if extra != nil {
		extra(req)
	}
	w := httptest.NewRecorder()
	status, err := webDAVHandler(w, req, &requestContext{User: user})
	if status != 0 && status != http.StatusOK {
		if w.Code == 0 || w.Code == http.StatusOK {
			w.Code = status
		}
	}
	if err != nil && status == 0 {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return w.Code, status
}

func TestWebDAVReadOnlySourceEnforcement(t *testing.T) {
	source1Path, _ := setupWebDAVTestEnv(t)
	user := webDAVReadOnlyTestUser(source1Path)

	t.Run("read-only", func(t *testing.T) {
		setTestSourceReadOnly("source1", true)
		t.Cleanup(func() { setTestSourceReadOnly("source1", false) })

		writeCases := []struct {
			method string
			path   string
			setup  func(*http.Request)
		}{
			{method: "MKCOL", path: "/public/rodir"},
			{method: http.MethodPut, path: "/public/ro-new.txt"},
			{method: http.MethodDelete, path: "/public/readme.txt"},
			{
				method: "COPY",
				path:   "/public/readme.txt",
				setup: func(r *http.Request) {
					r.Header.Set("Destination", "/dav/source1/public/readme-copy.txt")
				},
			},
			{
				method: "MOVE",
				path:   "/public/readme.txt",
				setup: func(r *http.Request) {
					r.Header.Set("Destination", "/dav/source1/public/readme-moved.txt")
				},
			},
		}
		for _, tc := range writeCases {
			t.Run(tc.method, func(t *testing.T) {
				var body io.Reader
				if tc.method == http.MethodPut {
					body = strings.NewReader("data")
				}
				code, _ := callWebDAVHandler(t, user, tc.method, tc.path, body, tc.setup)
				if code != http.StatusForbidden {
					t.Fatalf("expected 403, got %d", code)
				}
			})
		}

		for _, tc := range []struct {
			method string
			path   string
		}{
			{method: "PROPFIND", path: "/public"},
			{method: http.MethodGet, path: "/public/readme.txt"},
		} {
			t.Run(tc.method+"_allowed", func(t *testing.T) {
				code, _ := callWebDAVHandler(t, user, tc.method, tc.path, nil, nil)
				if code == http.StatusForbidden {
					t.Fatalf("%s should not be forbidden on read-only source", tc.method)
				}
			})
		}
	})

	t.Run("writable", func(t *testing.T) {
		setTestSourceReadOnly("source1", false)
		code, _ := callWebDAVHandler(t, user, http.MethodPut, "/public/writable-new.txt", strings.NewReader("ok"), nil)
		if code != http.StatusCreated {
			t.Fatalf("PUT on writable source: expected 201, got %d", code)
		}
	})
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

func TestArchiveCreateDeniedOnReadOnlySource(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	setTestSourceReadOnly("uploads", true)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `{"fromSource":"uploads","paths":["/a.txt"],"destination":"/out.zip","format":"zip"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/archive", strings.NewReader(body))
	status, err := archiveCreateHandler(rec, req, &Context{User: user})
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("archive create on read-only: status=%d err=%v", status, err)
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
