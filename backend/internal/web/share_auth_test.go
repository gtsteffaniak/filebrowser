package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

const victimShareHash = "victim_hash_test_01"

func setupShareAuthTestUsers(t *testing.T) (owner, attacker, admin *users.User) {
	t.Helper()
	setupTestEnv(t)

	fullPerms := users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
	shareScope := users.BackendScope{Path: "/srv", Scope: "/", Permissions: fullPerms}
	backendPerms := map[string]users.SourceFilePermissions{"/srv": fullPerms}

	owner = &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "share_owner",
			Permissions: users.Permissions{Share: true},
		},
		BackendScopes:            []users.BackendScope{shareScope},
		BackendSourcePermissions: backendPerms,
		Version:                  users.SourcePermissionsMigrationVersion,
	}
	attacker = &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "share_attacker",
			Permissions: users.Permissions{Share: true},
		},
		BackendScopes:            []users.BackendScope{shareScope},
		BackendSourcePermissions: backendPerms,
		Version:                  users.SourcePermissionsMigrationVersion,
	}
	admin = &users.User{
		FrontendUser: users.FrontendUser{
			Username:    "share_admin",
			Permissions: users.Permissions{Admin: true, Share: true},
		},
		BackendScopes:            []users.BackendScope{shareScope},
		BackendSourcePermissions: backendPerms,
		Version:                  users.SourcePermissionsMigrationVersion,
	}

	for _, u := range []*users.User{owner, attacker, admin} {
		if err := state.CreateUser(u, ""); err != nil {
			t.Fatalf("CreateUser(%s): %v", u.Username, err)
		}
	}
	adminFetched, err := state.GetUserByUsername("share_admin")
	if err != nil {
		t.Fatalf("GetUserByUsername(share_admin): %v", err)
	}
	if err := state.UpdateUser(&users.User{
		ID: adminFetched.ID,
		FrontendUser: users.FrontendUser{
			Permissions: users.Permissions{Admin: true, Share: true},
		},
	}, "", "permissions"); err != nil {
		t.Fatalf("set admin permissions: %v", err)
	}

	for _, pair := range []struct {
		name string
		ptr  **users.User
	}{
		{"share_owner", &owner},
		{"share_attacker", &attacker},
		{"share_admin", &admin},
	} {
		got, err := state.GetUserByUsername(pair.name)
		if err != nil {
			t.Fatalf("GetUserByUsername(%s): %v", pair.name, err)
		}
		*pair.ptr = &got
	}

	return owner, attacker, admin
}

func createVictimShare(t *testing.T, ownerID uint64) {
	t.Helper()
	victim := &share.Share{
		ShareSettings: share.ShareSettings{
			FrontendShareInfo: share.FrontendShareInfo{ShareType: "normal"},
			ShareLimits:       share.ShareLimits{SourceName: "srv"},
		},
		ShareColumns: share.ShareColumns{
			Hash: victimShareHash,
			Path: "/",
		},
		SourcePath: "/srv",
		UserID:     ownerID,
		Version:    1,
	}
	if err := state.CreateShare(victim); err != nil {
		t.Fatalf("CreateShare: %v", err)
	}
}

func postShareUpdate(t *testing.T, user *users.User, hash string, allowDelete bool) (int, error) {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{
		"hash":        hash,
		"allowDelete": allowDelete,
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/share", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	return sharePostHandler(rec, req, &Context{User: user})
}

func TestSharePostUpdate_RejectNonOwner(t *testing.T) {
	owner, attacker, _ := setupShareAuthTestUsers(t)
	createVictimShare(t, owner.ID)

	status, err := postShareUpdate(t, attacker, victimShareHash, true)
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got status=%d err=%v", status, err)
	}

	got, err := state.GetShare(victimShareHash)
	if err != nil {
		t.Fatalf("GetShare: %v", err)
	}
	if got.AllowDelete {
		t.Fatal("non-owner update should not enable allowDelete")
	}
	if got.UserID != owner.ID {
		t.Fatalf("UserID changed to %d, want owner %d", got.UserID, owner.ID)
	}
}

func TestSharePostUpdate_AllowsOwner(t *testing.T) {
	owner, _, _ := setupShareAuthTestUsers(t)
	createVictimShare(t, owner.ID)

	status, err := postShareUpdate(t, owner, victimShareHash, true)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got status=%d err=%v", status, err)
	}

	got, err := state.GetShare(victimShareHash)
	if err != nil {
		t.Fatalf("GetShare: %v", err)
	}
	if !got.AllowDelete {
		t.Fatal("owner update should enable allowDelete")
	}
	if got.UserID != owner.ID {
		t.Fatalf("UserID changed to %d, want owner %d", got.UserID, owner.ID)
	}
}

func TestSharePostUpdate_AdminCanEdit(t *testing.T) {
	owner, _, admin := setupShareAuthTestUsers(t)
	createVictimShare(t, owner.ID)

	status, err := postShareUpdate(t, admin, victimShareHash, true)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got status=%d err=%v", status, err)
	}

	got, err := state.GetShare(victimShareHash)
	if err != nil {
		t.Fatalf("GetShare: %v", err)
	}
	if !got.AllowDelete {
		t.Fatal("admin update should enable allowDelete")
	}
	if got.UserID != owner.ID {
		t.Fatalf("admin update should not reassign UserID: got %d want %d", got.UserID, owner.ID)
	}
}
