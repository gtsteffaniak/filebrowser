package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/webdav"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

const bobSecretMarker = "BOB_SECRET_MARKER_symlink_deny"

// setupSymlinkDenyHTTPTest builds alice/bob layout with escape symlinks and a DENY on /bob.
func setupSymlinkDenyHTTPTest(t *testing.T) (sourceRoot, sourceName string, denied *users.User) {
	t.Helper()
	lockWebGlobals(t)
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "symlink-deny.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() { state.Close() })

	sourceRoot = filepath.Join(tempDir, "data")
	aliceDir := filepath.Join(sourceRoot, "alice")
	bobDir := filepath.Join(sourceRoot, "bob")
	for _, dir := range []string{aliceDir, bobDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bobDir, "secret.txt"), []byte("line1\n"+bobSecretMarker), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../bob/secret.txt", filepath.Join(aliceDir, "link.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../bob", filepath.Join(aliceDir, "bobdir")); err != nil {
		t.Fatal(err)
	}

	sourceName = "data"
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		sourceRoot: {Path: sourceRoot, Name: sourceName},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		sourceName: settings.Config.Server.SourceMap[sourceRoot],
	}
	settings.InitializeUserResolvers()

	perms := users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
	denied = &users.User{
		ID: 2,
		FrontendUser: users.FrontendUser{
			Username: "denied-bob",
		},
		BackendScopes: []users.BackendScope{{
			Path:  sourceRoot,
			Scope: "/",
		}},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			sourceRoot: perms,
		},
		Version: users.SourcePermissionsMigrationVersion,
	}
	if err := state.CreateUser(denied, ""); err != nil {
		t.Fatal(err)
	}
	if err := state.AllowUser(sourceRoot, utils.IndexPathFromNormalized("/", true), denied.Username); err != nil {
		t.Fatal(err)
	}
	if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/bob", true), denied.Username); err != nil {
		t.Fatal(err)
	}

	indexing.SetTestIndex(sourceName, sourceRoot)
	t.Cleanup(func() { indexing.ClearTestIndices() })

	return sourceRoot, sourceName, denied
}

func TestServeSingleFile_RejectsDeniedSymlinkTarget(t *testing.T) {
	_, sourceName, user := setupSymlinkDenyHTTPTest(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	status, err := ServeSingleFile(rec, req, &requestContext{User: user}, sourceName,
		"/alice/link.txt", "link.txt", ServeSingleFileOptions{})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if strings.Contains(rec.Body.String(), bobSecretMarker) {
		t.Fatal("response leaked denied target")
	}
}

func TestFileWatchHandler_RejectsDeniedSymlinkTarget(t *testing.T) {
	_, sourceName, user := setupSymlinkDenyHTTPTest(t)
	req := httptest.NewRequest(http.MethodGet, "/api/tools/file-watcher?source="+sourceName+"&path=alice/link.txt", nil)
	rec := httptest.NewRecorder()
	status, err := fileWatchHandler(rec, req, &requestContext{User: user})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v body=%q", status, err, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), bobSecretMarker) {
		t.Fatal("watcher leaked denied target")
	}
}

func TestResourcePostHandler_RejectsWriteViaDenySymlinkDir(t *testing.T) {
	sourceRoot, sourceName, user := setupSymlinkDenyHTTPTest(t)
	req := httptest.NewRequest(http.MethodPost, "/api/resources?source="+sourceName+"&path=/alice/bobdir/planted.txt&override=true", strings.NewReader("planted"))
	rec := httptest.NewRecorder()
	status, err := ResourcePostHandler(rec, req, &requestContext{User: user})
	if status != http.StatusForbidden {
		t.Fatalf("status=%d err=%v", status, err)
	}
	if _, statErr := os.Stat(filepath.Join(sourceRoot, "bob", "planted.txt")); !os.IsNotExist(statErr) {
		t.Fatal("upload escaped into denied folder via directory symlink")
	}
}

func TestResourcePatchHandler_RejectsCopyFromDeniedSymlink(t *testing.T) {
	sourceRoot, sourceName, user := setupSymlinkDenyHTTPTest(t)
	body, err := json.Marshal(MoveCopyRequest{
		Action: "copy",
		Items: []MoveCopyItem{{
			FromSource: sourceName,
			FromPath:   "/alice/link.txt",
			ToSource:   sourceName,
			ToPath:     "/stolen-secret.txt",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/resources", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	status, handlerErr := ResourcePatchHandler(rec, req, &requestContext{User: user})
	if handlerErr != nil {
		t.Fatalf("unexpected handler error: %v", handlerErr)
	}
	if status != http.StatusInternalServerError && status != http.StatusOK && status != http.StatusMultiStatus {
		t.Fatalf("status=%d", status)
	}
	if _, statErr := os.Stat(filepath.Join(sourceRoot, "stolen-secret.txt")); !os.IsNotExist(statErr) {
		t.Fatal("copy exfiltrated denied file")
	}
	var resp MoveCopyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Succeeded) != 0 {
		t.Fatalf("expected copy failure, succeeded=%v", resp.Succeeded)
	}
}

func TestWebDAV_OpenFile_RejectsReadViaDeniedSymlink(t *testing.T) {
	sourceRoot, sourceName, user := setupSymlinkDenyHTTPTest(t)
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		t.Fatal("missing index")
	}
	scopePath, _, err := idx.GetRealPath("")
	if err != nil {
		t.Fatal(err)
	}
	ffs := &filteredFileSystem{
		fs:        webdav.Dir(scopePath),
		source:    sourceName,
		user:      user,
		filePerms: user.BackendSourcePermissions[sourceRoot],
	}
	_, openErr := ffs.OpenFile(context.Background(), "/alice/link.txt", os.O_RDONLY, 0)
	if openErr == nil {
		t.Fatal("expected read through denied symlink to fail")
	}
	if !os.IsPermission(openErr) {
		t.Fatalf("open err=%v want permission error", openErr)
	}
}
