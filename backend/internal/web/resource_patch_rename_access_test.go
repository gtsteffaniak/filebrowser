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
)

func TestResourcePatchHandler_CopyRename_DeniesDeniedVersionSuffix(t *testing.T) {
	_, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
	destDir := filepath.Join(sourceRoot, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(sourceRoot, "src.txt")
	if err := os.WriteFile(src, []byte("src"), 0o644); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(destDir, "report.txt")
	if err := os.WriteFile(existing, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	denied := filepath.Join(destDir, "report(1).txt")
	if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/report(1).txt", false), username); err != nil {
		t.Fatal(err)
	}

	user, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	body := `{"action":"copy","rename":true,"items":[{"fromSource":"` + sourceName +
		`","fromPath":"/src.txt","toSource":"` + sourceName + `","toPath":"/dest/report.txt"}]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/resources", strings.NewReader(body))
	rec := httptest.NewRecorder()
	status, handlerErr := ResourcePatchHandler(rec, req, &Context{User: &user})
	if handlerErr != nil {
		t.Fatalf("err=%v", handlerErr)
	}
	if status != http.StatusInternalServerError && status != http.StatusOK {
		t.Fatalf("status=%d body=%s", status, rec.Body.String())
	}
	var resp MoveCopyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Failed) != 1 || !strings.Contains(resp.Failed[0].Message, "access denied") {
		t.Fatalf("failed=%+v", resp.Failed)
	}
	if _, err := os.Stat(denied); err == nil {
		t.Fatal("denied destination file should not exist")
	}
}
