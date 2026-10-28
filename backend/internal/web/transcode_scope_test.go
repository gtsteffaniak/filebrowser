package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// scopedTranscodeFixture builds a source rooted at <tmp> whose user scope is
// `scope`, with the file at <tmp>/Test/movie.mp4. API paths are scope-relative
// (finalizeResponse strips the scope), so the file is requested as "/movie.mp4".
//
// <tmp>/movie.mp4 is deliberately never created: resolving without joining the
// scope lands exactly there, which is the reported 500.
func scopedTranscodeFixture(t *testing.T, scope string) (root, realFile, sourceName string, user *users.User) {
	t.Helper()

	root = t.TempDir()
	realFile = filepath.Join(root, "Test", "movie.mp4")
	if err := os.MkdirAll(filepath.Dir(realFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(realFile, []byte("path resolution fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Resolution returns the symlink-free path (macOS /var -> /private/var).
	if resolved, err := filepath.EvalSymlinks(realFile); err == nil {
		realFile = resolved
	}

	sourceName = "Files"
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		root: {Path: root, Name: sourceName},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		sourceName: settings.Config.Server.SourceMap[root],
	}
	settings.InitializeUserResolvers()

	user = &users.User{
		ID:           1,
		FrontendUser: users.FrontendUser{Username: "scoped"},
		BackendScopes: []users.BackendScope{{
			Path:        root,
			Scope:       scope,
			Permissions: users.SourceFilePermissions{View: true, Download: true},
		}},
	}
	if err := state.CreateUser(user, ""); err != nil {
		t.Fatal(err)
	}

	indexing.SetTestIndex(sourceName, root)
	t.Cleanup(func() { indexing.ClearTestIndices() })

	return root, realFile, sourceName, user
}

// A scope other than "/" must resolve to the real file. GetRealPathScoped uses its
// bound only for the containment check, so the scope has to be joined onto the
// scope-relative API path first — otherwise the path lands one level above the
// file and lstat fails.
func TestResolveTranscodeFileJoinsUserScope(t *testing.T) {
	setupTestEnv(t)
	root, realFile, sourceName, user := scopedTranscodeFixture(t, "/Test")

	realPath, fileName, status, err := resolveTranscodeFile(&Context{User: user}, sourceName, "/movie.mp4")
	if err != nil {
		t.Fatalf("resolveTranscodeFile: %v (status %d)", err, status)
	}
	if realPath != realFile {
		t.Fatalf("realPath = %q, want %q", realPath, realFile)
	}
	if fileName != "movie.mp4" {
		t.Fatalf("fileName = %q, want movie.mp4", fileName)
	}
	if _, statErr := os.Stat(filepath.Join(root, "movie.mp4")); !os.IsNotExist(statErr) {
		t.Fatal("fixture must not contain the un-joined path")
	}
}

// A scope of "/" is the identity case and must not double-join.
func TestResolveTranscodeFileRootScopeUnchanged(t *testing.T) {
	setupTestEnv(t)
	_, realFile, sourceName, user := scopedTranscodeFixture(t, "/")

	realPath, _, status, err := resolveTranscodeFile(&Context{User: user}, sourceName, "/Test/movie.mp4")
	if err != nil {
		t.Fatalf("resolveTranscodeFile: %v (status %d)", err, status)
	}
	if realPath != realFile {
		t.Fatalf("realPath = %q, want %q", realPath, realFile)
	}
}

// The access-rule check must run against the full index path. A deny rule on
// /Test/movie.mp4 has to block a request for the scope-relative "/movie.mp4";
// checking the un-joined path would consult no rule and allow it.
func TestResolveTranscodeFileAccessCheckUsesFullIndexPath(t *testing.T) {
	setupTestEnv(t)
	root, _, sourceName, user := scopedTranscodeFixture(t, "/Test")

	if err := state.DenyUser(root, utils.IndexPathFromNormalized("/Test/movie.mp4", false), user.Username); err != nil {
		t.Fatal(err)
	}

	_, _, status, err := resolveTranscodeFile(&Context{User: user}, sourceName, "/movie.mp4")
	if err == nil {
		t.Fatal("expected the deny rule on /Test/movie.mp4 to block the request")
	}
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (err=%v)", status, err)
	}
}

// Traversal must not reach a file that sits inside the source root but outside
// the user's scope, and in-scope files must still resolve.
func TestResolveTranscodeFileStaysInsideUserScope(t *testing.T) {
	setupTestEnv(t)
	root, _, sourceName, user := scopedTranscodeFixture(t, "/Test")

	// Inside the source root, outside the /Test scope.
	if err := os.WriteFile(filepath.Join(root, "outside.mp4"), []byte("out of scope"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Positive control inside the scope.
	inside := filepath.Join(root, "Test", "inside.mp4")
	if err := os.WriteFile(inside, []byte("in scope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if resolved, err := filepath.EvalSymlinks(inside); err == nil {
		inside = resolved
	}

	got, _, status, err := resolveTranscodeFile(&Context{User: user}, sourceName, "/inside.mp4")
	if err != nil || got != inside {
		t.Fatalf("in-scope file: realPath = %q (status %d, err %v), want %q", got, status, err, inside)
	}

	for _, attempt := range []string{"/../outside.mp4", "/../../outside.mp4", "/Test/../outside.mp4"} {
		got, _, status, err := resolveTranscodeFile(&Context{User: user}, sourceName, attempt)
		if err == nil {
			t.Fatalf("%q escaped the user scope: %q", attempt, got)
		}
		if status != http.StatusForbidden && status != http.StatusInternalServerError {
			t.Fatalf("%q: status = %d, want 403/500 (err=%v)", attempt, status, err)
		}
	}
}

// A user with no scope for the source must be rejected outright.
func TestResolveTranscodeFileRequiresScope(t *testing.T) {
	setupTestEnv(t)
	_, _, sourceName, user := scopedTranscodeFixture(t, "/Test")
	user.BackendScopes = nil

	_, _, status, err := resolveTranscodeFile(&Context{User: user}, sourceName, "/movie.mp4")
	if err == nil {
		t.Fatal("expected an error for a user without a scope on the source")
	}
	if status != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (err=%v)", status, err)
	}
}

// End to end: a scoped source must start a transcode session. Before the scope
// join this returned 500 ("could not resolve symlinks ... no such file or
// directory") and no session was created.
func TestTranscodeStartSucceedsForScopedSource(t *testing.T) {
	setupTestEnv(t)
	mgr, cleanup := setupTranscodeHandlerTest(t)
	defer cleanup()

	_, _, sourceName, user := scopedTranscodeFixture(t, "/Test")

	d := &Context{User: user}
	token, err := mintViewGrant(d, sourceName)
	if err != nil {
		t.Fatalf("mintViewGrant: %v", err)
	}

	body := `{"source":"` + sourceName + `","path":"/movie.mp4","profile":"quality","viewToken":"` + token + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/media/transcode/sessions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	status, err := transcodeStartHandler(rec, req, d)
	if err != nil {
		t.Fatalf("transcodeStartHandler: %v (status %d)", err, status)
	}
	if status != http.StatusCreated {
		t.Fatalf("status = %d, want 201", status)
	}
	if snap := mgr.Snapshot(user.Username, false, 1); len(snap.Sessions) != 1 {
		t.Fatalf("expected one active session, got %d", len(snap.Sessions))
	}
}
