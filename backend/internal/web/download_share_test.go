package web

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupShareArchiveDownloadTest(t *testing.T) (sourceRoot string, sourceName string, owner *users.User) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "share-dl-test.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() { state.Close() })

	sourceRoot = t.TempDir()
	sharedDir := filepath.Join(sourceRoot, "shared")
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "hello.txt"), []byte("share-zip-contents"), 0o644); err != nil {
		t.Fatal(err)
	}

	sourceName = "data"
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		sourceRoot: {
			Path: sourceRoot,
			Name: sourceName,
			Config: settings.SourceConfig{
				DenyByDefault: true,
			},
		},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		sourceName: settings.Config.Server.SourceMap[sourceRoot],
	}
	settings.InitializeUserResolvers()

	owner = &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username: "owner",
		},
	}
	if err := state.CreateUser(owner, ""); err != nil {
		t.Fatal(err)
	}
	if err := state.AllowUser(sourceRoot, utils.IndexPathFromNormalized("/shared", true), "owner"); err != nil {
		t.Fatal(err)
	}

	indexing.SetTestIndex(sourceName, sourceRoot)
	t.Cleanup(func() { indexing.ClearTestIndices() })

	return sourceRoot, sourceName, owner
}

func shareArchiveContext(owner *users.User, sharePath string) *Context {
	return &Context{
		User: &users.User{
			FrontendUser: users.FrontendUser{Username: users.AnonymousUserName},
		},
		ShareUser: owner,
		Share: share.Share{
			ShareColumns: share.ShareColumns{
				Hash: "testshare",
				Path: sharePath,
			},
		},
	}
}

func readZipFileNames(data []byte) []string {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}

func TestCreateZipPublicShareUsesShareOwnerAccessRules(t *testing.T) {
	_, sourceName, owner := setupShareArchiveDownloadTest(t)

	d := shareArchiveContext(owner, "/shared")

	var buf bytes.Buffer
	if err := createZip(d, sourceName, &buf, "/shared"); err != nil {
		t.Fatalf("createZip: %v", err)
	}
	names := readZipFileNames(buf.Bytes())
	if len(names) == 0 {
		t.Fatal("expected non-empty zip for public share archive (owner has allow rule)")
	}
	found := false
	for _, n := range names {
		if n == "shared/hello.txt" || n == "hello.txt" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("zip entries %v should include shared file", names)
	}

	// Without ShareUser, anonymous is denied under denyByDefault → empty archive.
	dBroken := &Context{
		User: &users.User{
			FrontendUser: users.FrontendUser{Username: users.AnonymousUserName},
		},
		Share: share.Share{
			ShareColumns: share.ShareColumns{
				Hash: "testshare",
				Path: "/shared",
			},
		},
	}
	var emptyBuf bytes.Buffer
	if err := createZip(dBroken, sourceName, &emptyBuf, "/shared"); err != nil {
		t.Fatalf("createZip: %v", err)
	}
	if len(readZipFileNames(emptyBuf.Bytes())) != 0 {
		t.Fatal("expected empty zip when access is evaluated as anonymous only")
	}
}

func TestComputeArchiveSizePublicShareUsesShareOwner(t *testing.T) {
	_, sourceName, _ := setupShareArchiveDownloadTest(t)

	dBroken := &Context{
		User: &users.User{
			FrontendUser: users.FrontendUser{Username: users.AnonymousUserName},
		},
		Share: share.Share{
			ShareColumns: share.ShareColumns{
				Hash: "testshare",
				Path: "/shared",
			},
		},
	}
	sizeBroken, err := computeArchiveSize(sourceName, []string{"/shared"}, dBroken)
	if err != nil {
		t.Fatalf("computeArchiveSize: %v", err)
	}
	if sizeBroken != 0 {
		t.Fatalf("expected zero estimated size for anonymous-only access, got %d", sizeBroken)
	}
}

func TestBuildAndStreamArchivePublicShareHEADProducesArchive(t *testing.T) {
	_, sourceName, owner := setupShareArchiveDownloadTest(t)
	settings.Config.Server.CacheDir = t.TempDir()
	if err := os.MkdirAll(settings.DownloadCacheDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	settings.Config.Server.MaxArchiveSizeGB = 0

	d := shareArchiveContext(owner, "/shared")
	req := httptest.NewRequest(http.MethodHead, "/?algo=zip", nil)
	rec := httptest.NewRecorder()

	status, err := BuildAndStreamArchive(rec, req, d, sourceName, []string{"/shared"})
	if err != nil {
		t.Fatalf("BuildAndStreamArchive HEAD: status=%d err=%v", status, err)
	}
	cl := rec.Header().Get("Content-Length")
	if cl == "" || cl == "0" {
		t.Fatalf("expected non-zero Content-Length on HEAD archive, got %q", cl)
	}
	token := rec.Header().Get("X-Archive-Token")
	if token == "" {
		t.Fatal("expected X-Archive-Token for chunked archive session")
	}
}

func TestRawFilesHandlerShareDirectoryDownloadNotEmptyZip(t *testing.T) {
	sourceRoot, sourceName, owner := setupShareArchiveDownloadTest(t)
	settings.Config.Server.NameToSource[sourceName].Path = sourceRoot

	d := shareArchiveContext(owner, "/shared")
	req := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash=testshare&file=%2F&algo=zip", nil)
	rec := httptest.NewRecorder()

	status, err := RawFilesHandler(rec, req, d, sourceName, []string{"/shared"})
	if err != nil {
		t.Fatalf("RawFilesHandler: status=%d err=%v", status, err)
	}
	if status != http.StatusOK && status != 0 {
		t.Fatalf("unexpected status %d", status)
	}
	body, readErr := io.ReadAll(rec.Body)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(readZipFileNames(body)) == 0 {
		t.Fatal("expected non-empty zip body from share directory download")
	}
}

func TestRawFilesHandler_MissingFileReturns404(t *testing.T) {
	sourceRoot, sourceName, _ := setupShareArchiveDownloadTest(t)
	ownerUser := users.User{
		FrontendUser: users.FrontendUser{Username: "owner"},
		BackendScopes: []users.BackendScope{{
			Path:  sourceRoot,
			Scope: "/",
			Permissions: users.SourceFilePermissions{
				View: true, Download: true, Modify: true, Create: true, Delete: true,
			},
		}},
	}
	users.SyncBackendSourcePermissionsMap(&ownerUser)

	d := &Context{User: &ownerUser}
	req := httptest.NewRequest(http.MethodGet, "/api/resources/download?source="+sourceName+"&file=/shared/missing.txt", nil)
	rec := httptest.NewRecorder()

	status, err := RawFilesHandler(rec, req, d, sourceName, []string{"/shared/missing.txt"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got status=%d err=%v", status, err)
	}
}

func TestPublicDownload_MissingFileReturns404(t *testing.T) {
	sourceRoot, _, owner := setupShareArchiveDownloadTest(t)
	d := shareArchiveContext(owner, "/shared")
	d.Share.SourcePath = sourceRoot

	req := httptest.NewRequest(http.MethodGet, "/public/api/resources/download?hash=testshare&file=missing.txt", nil)
	rec := httptest.NewRecorder()

	status, err := publicDownloadHandler(rec, req, d)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got status=%d err=%v", status, err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "error processing filelist") {
		t.Fatalf("expected masked share error, got %q", msg)
	}
	if strings.Contains(msg, sourceRoot) {
		t.Fatalf("share error should not leak host path, got %q", msg)
	}
}

func TestBuildAndStreamArchive_MissingPathReturns404(t *testing.T) {
	_, sourceName, owner := setupShareArchiveDownloadTest(t)
	d := shareArchiveContext(owner, "/shared")
	req := httptest.NewRequest(http.MethodGet, "/?algo=zip", nil)
	rec := httptest.NewRecorder()

	status, err := BuildAndStreamArchive(rec, req, d, sourceName, []string{"/shared/missing.txt"})
	if err == nil {
		t.Fatal("expected error for missing archive path")
	}
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got status=%d err=%v", status, err)
	}
}

func TestServeSingleFile_MissingScopedPathReturns404(t *testing.T) {
	sourceRoot, sourceName, _ := setupShareArchiveDownloadTest(t)
	ownerUser := users.User{
		FrontendUser: users.FrontendUser{Username: "owner"},
		BackendScopes: []users.BackendScope{{
			Path:  sourceRoot,
			Scope: "/",
			Permissions: users.SourceFilePermissions{
				View: true, Download: true, Modify: true, Create: true, Delete: true,
			},
		}},
	}
	users.SyncBackendSourcePermissionsMap(&ownerUser)

	d := &Context{User: &ownerUser}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	status, err := ServeSingleFile(rec, req, d, sourceName, "/shared/missing.txt", "missing.txt", ServeSingleFileOptions{})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got status=%d err=%v", status, err)
	}
}
