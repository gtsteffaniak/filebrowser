package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestArchiveCreate_DeniesResolvedSymlinkDestination(t *testing.T) {
	_, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
	if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/denied", true), username); err != nil {
		t.Fatal(err)
	}
	allowedDir := filepath.Join(sourceRoot, "allowed")
	deniedDir := filepath.Join(sourceRoot, "denied")
	for _, dir := range []string{allowedDir, deniedDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	victim := filepath.Join(deniedDir, "victim")
	if err := os.WriteFile(victim, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceRoot, "a.txt"), []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(allowedDir, "report.zip")
	if err := os.Symlink(filepath.Join("..", "denied", "victim"), link); err != nil {
		t.Fatal(err)
	}

	user, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	settings.Config.Server.SourceMap[sourceRoot].Config.DenyByDefault = false

	body := `{"fromSource":"` + sourceName + `","paths":["/a.txt"],"destination":"/allowed/report.zip","format":"zip"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/archive", strings.NewReader(body))
	rec := httptest.NewRecorder()
	status, handlerErr := archiveCreateHandler(rec, req, &Context{User: &user})
	if status != http.StatusForbidden || handlerErr == nil {
		t.Fatalf("status=%d err=%v", status, handlerErr)
	}
	got, err := os.ReadFile(victim)
	if err != nil || string(got) != "keep" {
		t.Fatalf("victim content=%q err=%v", got, err)
	}
}

func TestArchiveCreate_AllowsNonSymlinkDestination(t *testing.T) {
	_, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
	dest := filepath.Join(sourceRoot, "out.zip")
	if err := os.WriteFile(filepath.Join(sourceRoot, "a.txt"), []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	user, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"fromSource":"` + sourceName + `","paths":["/a.txt"],"destination":"/out.zip","format":"zip"}`
	req := httptest.NewRequest(http.MethodPost, "/api/resources/archive", strings.NewReader(body))
	rec := httptest.NewRecorder()
	status, handlerErr := archiveCreateHandler(rec, req, &Context{User: &user})
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("status=%d err=%v body=%s", status, handlerErr, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("expected archive at %s: %v", dest, err)
	}
}
