package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/preview"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

func TestResourcePostHandler_RejectsEscapingParentSymlinkMkdir(t *testing.T) {
	downloadsPath := setupPutHTTPTest(t)
	outside := filepath.Join(filepath.Dir(downloadsPath), "outside-mkdir")
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

	req := httptest.NewRequest(http.MethodPost, "/api/resources?source=Downloads&path=/evil/newdir&isDir=true", nil)
	rec := httptest.NewRecorder()

	status, handlerErr := ResourcePostHandler(rec, req, &requestContext{User: &adminUser})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v", status, handlerErr)
	}
	if handlerErr == nil {
		t.Fatal("expected mkdir to be forbidden")
	}
	if _, err := os.Stat(filepath.Join(outside, "newdir")); !os.IsNotExist(err) {
		t.Fatal("mkdir escaped outside the source via symlink parent")
	}
}

func TestResourceDeleteHandler_RejectsEscapingIntermediate(t *testing.T) {
	downloadsPath := setupPutHTTPTest(t)
	if preview.GetService() == nil {
		if err := preview.StartPreviewGenerator(1, t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	outsideDir := filepath.Join(filepath.Dir(downloadsPath), "outside-delete")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(outsideDir, "victim.txt")
	if err := os.WriteFile(victim, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(downloadsPath, "evil")); err != nil {
		t.Fatal(err)
	}

	adminUser, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/resources?source=Downloads&path=/evil/victim.txt", nil)
	rec := httptest.NewRecorder()

	status, handlerErr := resourceDeleteHandler(rec, req, &requestContext{User: &adminUser})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v", status, handlerErr)
	}
	if handlerErr == nil {
		t.Fatal("expected delete to be forbidden")
	}
	got, err := os.ReadFile(victim)
	if err != nil || string(got) != "keep" {
		t.Fatalf("delete escaped: err=%v content=%q", err, got)
	}
}
