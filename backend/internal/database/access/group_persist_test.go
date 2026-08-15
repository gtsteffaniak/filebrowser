package access_test

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

type stubGroupSQL struct {
	saveErr   error
	deleteErr error
}

func (s *stubGroupSQL) SaveAccessRule(string, string, *access.AccessRule) error { return nil }
func (s *stubGroupSQL) DeleteAccessRule(string, string) error                  { return nil }
func (s *stubGroupSQL) SaveGroup(string, access.StringSet) error {
	return s.saveErr
}
func (s *stubGroupSQL) DeleteGroup(string) error { return s.deleteErr }
func (s *stubGroupSQL) SaveRevokedToken(string) error {
	return nil
}
func (s *stubGroupSQL) SaveHashedToken(string, uint64) error { return nil }
func (s *stubGroupSQL) DeleteHashedToken(string) error       { return nil }
func (s *stubGroupSQL) DeleteHashedTokensByUserID(uint64) error {
	return nil
}

func TestAddUserToGroup_RollsBackOnSQLFailure(t *testing.T) {
	userStore := users.NewStorage(nil)
	store := access.NewStorage(userStore)
	store.SetSQLStore(&stubGroupSQL{saveErr: errors.New("save failed")})

	err := store.AddUserToGroup("editors", "alice")
	if err == nil {
		t.Fatal("expected SQL save error")
	}
	groups := store.GetUserGroups("alice")
	if len(groups) != 0 {
		t.Fatalf("expected in-memory rollback, alice groups = %#v", groups)
	}
}

func TestRemoveUserFromGroup_RollsBackOnSQLFailure(t *testing.T) {
	userStore := users.NewStorage(nil)
	store := access.NewStorage(userStore)
	if err := store.AddUserToGroup("editors", "alice"); err != nil {
		t.Fatalf("seed group: %v", err)
	}

	store.SetSQLStore(&stubGroupSQL{saveErr: errors.New("save failed")})
	err := store.RemoveUserFromGroup("editors", "alice")
	if err == nil {
		t.Fatal("expected SQL save error")
	}
	groups := store.GetUserGroups("alice")
	if len(groups) != 1 || groups[0] != "editors" {
		t.Fatalf("expected in-memory rollback, alice groups = %#v", groups)
	}
}

func TestAllowGroup_ReturnsSQLSaveError(t *testing.T) {
	setupTestSources()
	store := access.NewStorage(users.NewStorage(nil))
	store.SetSQLStore(&stubGroupSQL{saveErr: errors.New("save failed")})

	err := store.AllowGroup("mnt/storage", idxPath("/tenant"), "acme")
	if err == nil {
		t.Fatal("expected SQL save error from AllowGroup")
	}
}
