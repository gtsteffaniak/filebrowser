package access_test

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

func TestNormalizeGroupName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"trims", "  editors  ", "editors", false},
		{"spaces inside", "cn=admins with spaces,ou=x", "cn=admins with spaces,ou=x", false},
		{"empty", "", "", true},
		{"blank", "   ", "", true},
		{"newline", "a\nb", "", true},
		{"too long", string(make([]byte, access.MaxGroupNameLength+1)), "", true},
	} {
		got, err := access.NormalizeGroupName(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%s: expected error, got %q", tc.name, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("%s: got %q, err=%v", tc.name, got, err)
		}
	}
	// Long names of valid characters fail on length, not on content.
	if _, err := access.NormalizeGroupName(string(make([]byte, access.MaxGroupNameLength+1))); err == nil {
		t.Fatal("expected too-long name to fail")
	}
	if got := access.NormalizeMembers([]string{" a ", "b", "", "a", "  "}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected members: %v", got)
	}
}

func TestRemoveUserFromAllGroups(t *testing.T) {
	setupTestSources()
	s, _, sqlStore := createTestStorageWithSQL(t)

	if err := s.SetGroupMembers("editors", []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGroupMembers("staff", []string{"alice"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveUserFromAllGroups("alice"); err != nil {
		t.Fatal(err)
	}
	if groups := s.GetUserGroups("alice"); len(groups) != 0 {
		t.Fatalf("alice should be in no groups, got %v", groups)
	}
	if got := s.GetGroupMembers()["editors"]; len(got) != 1 || got[0] != "bob" {
		t.Fatalf("expected [bob], got %v", got)
	}
	// The emptied group row still exists.
	if _, ok := s.GetGroupMembers()["staff"]; !ok {
		t.Fatal("empty group should remain")
	}
	sqlGroups, err := sqlStore.GetAllGroups()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sqlGroups["staff"]["alice"]; ok {
		t.Fatal("alice should be removed from staff in SQL")
	}
	// Removing a user who is in no group is a no-op.
	if err := s.RemoveUserFromAllGroups("nobody"); err != nil {
		t.Fatal(err)
	}
}

func TestRenameUserInGroups(t *testing.T) {
	setupTestSources()
	s, _, sqlStore := createTestStorageWithSQL(t)

	if err := s.SetGroupMembers("editors", []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameUserInGroups("alice", "alicia"); err != nil {
		t.Fatal(err)
	}
	got := s.GetGroupMembers()["editors"]
	if len(got) != 2 || got[0] != "alicia" || got[1] != "bob" {
		t.Fatalf("unexpected members: %v", got)
	}
	sqlGroups, err := sqlStore.GetAllGroups()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sqlGroups["editors"]["alicia"]; !ok {
		t.Fatal("alicia should be persisted in SQL")
	}
	if _, ok := sqlGroups["editors"]["alice"]; ok {
		t.Fatal("alice should be removed in SQL")
	}
	// Renaming a non-member or same name is a no-op.
	if err := s.RenameUserInGroups("nobody", "other"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameUserInGroups("alicia", "alicia"); err != nil {
		t.Fatal(err)
	}
}
