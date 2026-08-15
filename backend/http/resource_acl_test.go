package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

func setupResourceACLTestSource(t *testing.T) string {
	t.Helper()
	srvRoot := t.TempDir()
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		srvRoot: {Path: srvRoot, Name: "srv"},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"srv": {Path: srvRoot, Name: "srv"},
	}
	indexing.SetTestIndex("srv", srvRoot)
	t.Cleanup(indexing.ClearTestIndices)
	return srvRoot
}

func TestResourcePostACLUsesFullIndexPathKey(t *testing.T) {
	setupTestEnv(t)

	const (
		sourcePath   = "/srv"
		userScope    = "/home/alice"
		deniedFolder = "/home/alice/projects/acme/private"
		relPath      = "/projects/acme/private/poison.docx"
	)
	fullIndexPath := utils.JoinScopedIndexPath(userScope, relPath)

	alice := &users.User{
		ID:       1,
		Username: "alice",
	}
	if err := store.Users.Save(alice, false, false); err != nil {
		t.Fatal(err)
	}

	if err := store.Access.DenyUser(sourcePath, deniedFolder, "alice"); err != nil {
		t.Fatalf("DenyUser: %v", err)
	}

	if !store.Access.Permitted(sourcePath, relPath, "alice") {
		t.Fatal("scope-stripped path does not match deny rule keyed at full index path (documents pre-fix ACL miss)")
	}
	if store.Access.Permitted(sourcePath, fullIndexPath, "alice") {
		t.Fatalf("full index path %q should be denied for alice", fullIndexPath)
	}
}

func TestResourcePostHandler_DeniesAuthenticatedUploadToDeniedPath(t *testing.T) {
	setupTestEnv(t)
	sourcePath := setupResourceACLTestSource(t)

	alice := &users.User{
		ID:          1,
		Username:    "alice",
		Permissions: users.Permissions{Create: true},
		Scopes: []users.SourceScope{
			{Name: sourcePath, Scope: "/home/alice"},
		},
	}
	if err := store.Users.Save(alice, false, false); err != nil {
		t.Fatal(err)
	}

	deniedFolder := "/home/alice/projects/acme/private"
	if err := store.Access.DenyUser(sourcePath, deniedFolder, "alice"); err != nil {
		t.Fatalf("DenyUser: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/resources?source=srv&path=/projects/acme/private/evil.txt",
		strings.NewReader("payload"),
	)
	rec := httptest.NewRecorder()

	status, err := resourcePostHandler(rec, req, &requestContext{user: alice})
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for denied upload path, got status=%d err=%v", status, err)
	}
}

func TestResourcePauseHandler_DeniesAuthenticatedPauseOnDeniedPath(t *testing.T) {
	setupTestEnv(t)
	sourcePath := setupResourceACLTestSource(t)

	alice := &users.User{
		ID:          1,
		Username:    "alice",
		Permissions: users.Permissions{Create: true},
		Scopes: []users.SourceScope{
			{Name: sourcePath, Scope: "/home/alice"},
		},
	}
	if err := store.Users.Save(alice, false, false); err != nil {
		t.Fatal(err)
	}

	if err := store.Access.DenyUser(sourcePath, "/home/alice/projects/acme/private", "alice"); err != nil {
		t.Fatalf("DenyUser: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/resources/pause?source=srv&path=/projects/acme/private/evil.txt",
		http.NoBody,
	)
	rec := httptest.NewRecorder()

	status, err := resourcePauseHandler(rec, req, &requestContext{user: alice})
	if status != http.StatusForbidden {
		t.Fatalf("expected 403 for denied pause path, got status=%d err=%v", status, err)
	}
}
