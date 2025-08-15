package access_test

import (
	"path/filepath"
	"testing"

	"github.com/asdine/storm/v3"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	boltusers "github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func createTestStorage(t *testing.T) (*access.Storage, *users.Storage) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := storm.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open storm db: %v", err)
	}
	userStore := users.NewStorage(boltusers.NewUsersBackend(db))
	return access.NewStorage(db, userStore), userStore
}

func createTestUser(t *testing.T, userStore *users.Storage, username string) {
	u := &users.User{NonAdminEditable: users.NonAdminEditable{Password: "test"}, Username: username}
	err := userStore.Save(u, false, false)
	if err != nil {
		t.Fatalf("failed to create user %s: %v", username, err)
	}
}

func TestPermitted_UserBlacklist(t *testing.T) {
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if err := s.DenyUser("mnt/storage", "/secret", "alice"); err != nil {
		t.Errorf("DenyUser failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/secret", "alice") {
		t.Error("alice should not be permitted (denied)")
	}
	if !s.Permitted("mnt/storage", "/secret", "bob") {
		t.Error("bob should be permitted (not denied)")
	}
}

func TestPermitted_UserWhitelist(t *testing.T) {
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/vip", "bob"); err != nil {
		t.Errorf("AllowUser failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/vip", "bob") {
		t.Error("bob should be permitted (allowed)")
	}
	if s.Permitted("mnt/storage", "/vip", "alice") {
		t.Error("alice should not be permitted (not allowed)")
	}
}

func TestPermitted_GroupBlacklist(t *testing.T) {
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	_ = s.AddUserToGroup("admins", "alice")
	if err := s.DenyGroup("mnt/storage", "/admin", "admins"); err != nil {
		t.Errorf("DenyGroup failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/admin", "bob") == false {
		t.Error("bob should be permitted (not in denied group)")
	}
	if s.Permitted("mnt/storage", "/admin", "alice") {
		t.Error("alice should not be permitted (in denied group)")
	}
}

func TestPermitted_GroupWhitelist(t *testing.T) {
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	_ = s.AddUserToGroup("vip", "bob")
	if err := s.AllowGroup("mnt/storage", "/vip", "vip"); err != nil {
		t.Errorf("AllowGroup failed: %v", err)
	}
	if !s.Permitted("mnt/storage", "/vip", "bob") {
		t.Error("bob should be permitted (in allowed group)")
	}
	if s.Permitted("mnt/storage", "/vip", "alice") {
		t.Error("alice should not be permitted (not in allowed group)")
	}
}

func TestPermitted_NoRule(t *testing.T) {
	s, _ := createTestStorage(t)
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	if !s.Permitted("mnt/storage", "/public", "anyone") {
		t.Error("anyone should be permitted if no rule exists")
	}
}

func TestPermitted_CombinedRules(t *testing.T) {
	s, userStore := createTestStorage(t)
	createTestUser(t, userStore, "alice")
	createTestUser(t, userStore, "bob")
	createTestUser(t, userStore, "carol")
	createTestUser(t, userStore, "eve")
	err := s.LoadFromDB()
	if err != nil && err != storm.ErrNotFound {
		t.Errorf("unexpected error loading from DB: %v", err)
	}
	err = s.AddUserToGroup("vip", "bob")
	if err != nil {
		t.Errorf("AddUserToGroup failed: %v", err)
	}
	err = s.AddUserToGroup("admins", "alice")
	if err != nil {
		t.Errorf("AddUserToGroup failed: %v", err)
	}
	if err := s.DenyUser("mnt/storage", "/combo", "eve"); err != nil {
		t.Errorf("DenyUser failed: %v", err)
	}
	if err := s.AllowUser("mnt/storage", "/combo", "carol"); err != nil {
		t.Errorf("AllowUser failed: %v", err)
	}
	if err := s.DenyGroup("mnt/storage", "/combo", "admins"); err != nil {
		t.Errorf("DenyGroup failed: %v", err)
	}
	if err := s.AllowGroup("mnt/storage", "/combo", "vip"); err != nil {
		t.Errorf("AllowGroup failed: %v", err)
	}
	if s.Permitted("mnt/storage", "/combo", "eve") {
		t.Error("eve should not be permitted (user denied)")
	}
	if !s.Permitted("mnt/storage", "/combo", "carol") {
		t.Error("carol should be permitted (user allowed)")
	}
	if s.Permitted("mnt/storage", "/combo", "alice") {
		t.Error("alice should not be permitted (in group denied)")
	}
	if !s.Permitted("mnt/storage", "/combo", "bob") {
		t.Error("bob should be permitted (in group allowed)")
	}
}
