package sqldb

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

// newGroupTestStore opens a fresh SQLStore in a temp dir.
func newGroupTestStore(t *testing.T) *SQLStore {
	t.Helper()
	store, _, err := NewSQLStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("failed to create SQL store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

// groupRule returns a rule that allows the given group.
func groupRule(group string) *access.AccessRule {
	return &access.AccessRule{Allow: access.RuleSet{
		Users:  access.StringSet{},
		Groups: access.StringSet{group: {}},
	}}
}

func TestDeleteGroupWithRules_CommitsAllChanges(t *testing.T) {
	store := newGroupTestStore(t)
	keep := access.RuleKey{Source: "src", Path: "/keep"}
	drop := access.RuleKey{Source: "src", Path: "/drop"}
	if err := store.SaveGroup("acme", access.StringSet{"alice": {}}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAccessRule(keep.Source, keep.Path, groupRule("acme")); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAccessRule(drop.Source, drop.Path, groupRule("acme")); err != nil {
		t.Fatal(err)
	}

	// keep is rewritten (now allows "other"), drop is removed, the group row is deleted.
	err := store.DeleteGroupWithRules("acme", []access.RuleUpsert{{RuleKey: keep, Rule: groupRule("other")}}, []access.RuleKey{drop})
	if err != nil {
		t.Fatalf("DeleteGroupWithRules: %v", err)
	}

	groups, err := store.GetAllGroups()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := groups["acme"]; ok {
		t.Fatal("group row should be deleted")
	}
	rules, err := store.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := rules["src"]["/drop"]; ok {
		t.Fatal("dropped rule row should be deleted")
	}
	kept, ok := rules["src"]["/keep"]
	if !ok {
		t.Fatal("kept rule row should still exist")
	}
	if _, has := kept.Allow.Groups["other"]; !has {
		t.Fatalf("kept rule should have been rewritten, got %+v", kept.Allow.Groups)
	}
	// Deleting again (group row already absent) is not an error.
	if err := store.DeleteGroupWithRules("acme", nil, nil); err != nil {
		t.Fatalf("repeat delete should succeed: %v", err)
	}
}

func TestDeleteGroupWithRules_RollsBackOnFailure(t *testing.T) {
	store := newGroupTestStore(t)
	if err := store.SaveGroup("acme", access.StringSet{"alice": {}}); err != nil {
		t.Fatal(err)
	}
	// Make the rule statements fail so the transaction has to roll back.
	if _, err := store.db.Exec(`DROP TABLE access_rules`); err != nil {
		t.Fatal(err)
	}

	err := store.DeleteGroupWithRules("acme", []access.RuleUpsert{{RuleKey: access.RuleKey{Source: "src", Path: "/x"}, Rule: groupRule("other")}}, nil)
	if err == nil {
		t.Fatal("expected an error when the rule write fails")
	}

	groups, gErr := store.GetAllGroups()
	if gErr != nil {
		t.Fatal(gErr)
	}
	if _, ok := groups["acme"]; !ok {
		t.Fatal("group row must survive a failed transaction")
	}
}
