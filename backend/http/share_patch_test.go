package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

func TestSharePatchHandler_AnchorsPathToOwnerScope(t *testing.T) {
	setupTestEnv(t)

	srvRoot := t.TempDir()
	publicDir := filepath.Join(srvRoot, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "marker.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/srv": {Path: "/srv", Name: "srv"},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"srv": {Path: "/srv", Name: "srv"},
	}
	indexing.SetTestIndex("srv", srvRoot)
	t.Cleanup(indexing.ClearTestIndices)

	guest := &users.User{
		ID:       2,
		Username: "guest",
		Permissions: users.Permissions{
			Share: true,
		},
		Scopes: []users.SourceScope{
			{Name: "/srv", Scope: "/public"},
		},
	}
	if err := store.Users.Save(guest, false, false); err != nil {
		t.Fatal(err)
	}

	initialPath := "/public/"
	link := &share.Link{
		Hash:    "patch_scope_hash",
		UserID:  guest.ID,
		Version: 1,
		CommonShare: share.CommonShare{
			Path:   initialPath,
			Source: "/srv",
		},
	}
	if err := store.Share.Save(link); err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(map[string]string{
		"hash": link.Hash,
		"path": "/",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/share", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	status, handlerErr := sharePatchHandler(rec, req, &requestContext{user: guest})
	if status != http.StatusOK {
		t.Fatalf("sharePatchHandler status=%d err=%v", status, handlerErr)
	}

	updated, err := store.Share.GetByHash(link.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Path == "/" {
		t.Fatalf("share path escaped to source root: %q", updated.Path)
	}
	if updated.Path != initialPath {
		t.Fatalf("share path = %q, want %q", updated.Path, initialPath)
	}
}

func TestSharePatchHandler_AdminUpdateUsesOwnerScope(t *testing.T) {
	setupTestEnv(t)

	srvRoot := t.TempDir()
	publicDir := filepath.Join(srvRoot, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(publicDir, "marker.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/srv": {Path: "/srv", Name: "srv"},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"srv": {Path: "/srv", Name: "srv"},
	}
	indexing.SetTestIndex("srv", srvRoot)
	t.Cleanup(indexing.ClearTestIndices)

	owner := &users.User{
		ID:       2,
		Username: "guest",
		Permissions: users.Permissions{
			Share: true,
		},
		Scopes: []users.SourceScope{
			{Name: "/srv", Scope: "/public"},
		},
	}
	if err := store.Users.Save(owner, false, false); err != nil {
		t.Fatal(err)
	}

	admin := &users.User{
		ID:       1,
		Username: "admin",
		Permissions: users.Permissions{
			Admin: true,
			Share: true,
		},
		Scopes: []users.SourceScope{
			{Name: "/srv", Scope: "/"},
		},
	}
	if err := store.Users.Save(admin, false, false); err != nil {
		t.Fatal(err)
	}

	initialPath := "/public/"
	link := &share.Link{
		Hash:    "patch_admin_scope_hash",
		UserID:  owner.ID,
		Version: 1,
		CommonShare: share.CommonShare{
			Path:   initialPath,
			Source: "/srv",
		},
	}
	if err := store.Share.Save(link); err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(map[string]string{
		"hash": link.Hash,
		"path": "/",
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPatch, "/api/share", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	status, handlerErr := sharePatchHandler(rec, req, &requestContext{user: admin})
	if status != http.StatusOK {
		t.Fatalf("sharePatchHandler status=%d err=%v", status, handlerErr)
	}

	updated, err := store.Share.GetByHash(link.Hash)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Path == "/" {
		t.Fatalf("admin update escaped to source root using admin scope: %q", updated.Path)
	}
	if updated.Path != initialPath {
		t.Fatalf("share path = %q, want %q", updated.Path, initialPath)
	}
}
