package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

const victimShareHashX79q = "victim_share_x79q_01"

func setupShareAuthUsers(t *testing.T) (owner, attacker, admin *users.User) {
	t.Helper()
	setupTestEnv(t)

	owner = &users.User{
		ID:          1,
		Username:    "share_owner",
		Permissions: users.Permissions{Share: true, Delete: true, Create: true, Modify: true},
		Scopes:      []users.SourceScope{{Name: "/srv", Scope: "/"}},
	}
	attacker = &users.User{
		ID:          2,
		Username:    "share_attacker",
		Permissions: users.Permissions{Share: true, Delete: true, Create: true, Modify: true},
		Scopes:      []users.SourceScope{{Name: "/srv", Scope: "/"}},
	}
	noDelete := &users.User{
		ID:          3,
		Username:    "share_no_delete",
		Permissions: users.Permissions{Share: true, Delete: false, Create: true, Modify: true},
		Scopes:      []users.SourceScope{{Name: "/srv", Scope: "/"}},
	}
	admin = &users.User{
		ID:          4,
		Username:    "share_admin",
		Permissions: users.Permissions{Admin: true, Share: true, Delete: true},
		Scopes:      []users.SourceScope{{Name: "/srv", Scope: "/"}},
	}

	for _, u := range []*users.User{owner, attacker, noDelete, admin} {
		if err := store.Users.Save(u, false, false); err != nil {
			t.Fatalf("Save(%s): %v", u.Username, err)
		}
	}
	gotOwner, err := store.Users.Get(owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	gotAttacker, err := store.Users.Get(attacker.ID)
	if err != nil {
		t.Fatal(err)
	}
	gotAdmin, err := store.Users.Get(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	return gotOwner, gotAttacker, gotAdmin
}

func createVictimShareLink(t *testing.T, ownerID uint) {
	t.Helper()
	link := &share.Link{
		Hash:   victimShareHashX79q,
		UserID: ownerID,
		Version: 1,
		CommonShare: share.CommonShare{
			Path:        "/",
			Source:      "/srv",
			ShareType:   "normal",
			AllowDelete: false,
		},
	}
	if err := store.Share.Save(link); err != nil {
		t.Fatal(err)
	}
}

func postShareUpdate(t *testing.T, user *users.User, hash string, allowDelete bool) (int, error) {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{
		"hash":        hash,
		"allowDelete": allowDelete,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	return sharePostHandler(rec, req, &requestContext{user: user})
}

func postShareCreate(t *testing.T, user *users.User, allowDelete bool) (int, error) {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{
		"path":        "/",
		"source":      "srv",
		"allowDelete": allowDelete,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	return sharePostHandler(rec, req, &requestContext{user: user})
}

func TestSharePostUpdate_RejectNonOwner(t *testing.T) {
	owner, attacker, _ := setupShareAuthUsers(t)
	createVictimShareLink(t, owner.ID)

	status, err := postShareUpdate(t, attacker, victimShareHashX79q, true)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}

	got, err := store.Share.GetByHash(victimShareHashX79q)
	if err != nil {
		t.Fatal(err)
	}
	if got.AllowDelete {
		t.Fatal("non-owner update should not enable allowDelete")
	}
	if got.UserID != owner.ID {
		t.Fatalf("UserID changed to %d, want owner %d", got.UserID, owner.ID)
	}
}

func TestSharePostUpdate_AllowsOwner(t *testing.T) {
	owner, _, _ := setupShareAuthUsers(t)
	createVictimShareLink(t, owner.ID)

	status, err := postShareUpdate(t, owner, victimShareHashX79q, true)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got status=%d err=%v", status, err)
	}

	got, err := store.Share.GetByHash(victimShareHashX79q)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AllowDelete {
		t.Fatal("owner update should enable allowDelete")
	}
}

func TestSharePostCreate_ClampsAllowDeleteWithoutUserDeletePerm(t *testing.T) {
	setupTestEnv(t)
	indexing.SetTestIndex("srv", t.TempDir())
	t.Cleanup(indexing.ClearTestIndices)

	user := &users.User{
		ID:          5,
		Username:    "share_no_delete_create",
		Permissions: users.Permissions{Share: true, Delete: false, Create: true},
		Scopes:      []users.SourceScope{{Name: "/srv", Scope: "/"}},
	}
	if err := store.Users.Save(user, false, false); err != nil {
		t.Fatal(err)
	}
	got, err := store.Users.Get(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	status, handlerErr := postShareCreate(t, got, true)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got status=%d err=%v", status, handlerErr)
	}

	links, err := store.Share.GetBySourcePath("/", "/srv")
	if err != nil || len(links) == 0 {
		t.Fatalf("expected created share, err=%v len=%d", err, len(links))
	}
	created := links[len(links)-1]
	if created.AllowDelete {
		t.Fatal("user without delete permission should not persist allowDelete=true")
	}
}
