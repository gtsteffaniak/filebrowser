package sqldb

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

func TestMigrateAccessRulePathsLowercaseSetsMarkerInTransaction(t *testing.T) {
	store, _, err := NewSQLStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	rule := &access.AccessRule{
		Allow: access.RuleSet{Users: map[string]struct{}{"bob": {}}},
	}
	if err := store.SaveAccessRule("src", "/Old/", rule); err != nil {
		t.Fatal(err)
	}

	newRules := access.RuleMap{
		"/old/": rule,
	}
	if err := store.MigrateAccessRulePathsLowercase("src", []string{"/Old/"}, newRules); err != nil {
		t.Fatal(err)
	}
	ok, err := store.AccessRulesLowercaseMigrated()
	if err != nil || !ok {
		t.Fatalf("marker should be set after migrate, ok=%v err=%v", ok, err)
	}
}
