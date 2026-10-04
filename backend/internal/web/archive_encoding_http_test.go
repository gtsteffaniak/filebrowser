package web

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/archiveencoding"
)

func TestUnarchiveEncodingPreviewDoesNotWriteOrDelete(t *testing.T) {
	root, user := setupUploadHTTPTest(t)
	archivePath := filepath.Join(root, "test.zip")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: "\x83\x65\x83\x58\x83\x67.txt", NonUTF8: true})
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
		t.Fatalf("status=%d err=%v body=%s", status, err, rec.Body.String())
	}
	var preview archiveencoding.Preview
	if err = json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview.Suggested != "cp932" {
		t.Fatalf("preview: %+v", preview)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "test.zip" {
		t.Fatalf("preview modified source: %v, %v", entries, err)
	}

	// The preview goes through the same source permission checks as extraction.
	perms := user.BackendSourcePermissions[root]
	perms.Download = false
	user.BackendSourcePermissions[root] = perms
	user.BackendScopes[0].Permissions.Download = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/resources/unarchive", strings.NewReader(body))
	status, err = unarchiveHandler(rec, req, &Context{User: user})
	if status != http.StatusForbidden || err == nil {
		t.Fatalf("expected denied preview, got %d, %v", status, err)
	}
}
