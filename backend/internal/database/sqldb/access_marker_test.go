package sqldb

import (
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

// The lowercase marker is set by StartupMigrateAccessRuleKeys after all
// sources migrate, not inside the per-source migration transaction.
func TestMigrateAccessRulePathsLowercaseRewritesPaths(t *testing.T) {
	store, _, err := NewSQLStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	rule := &access.AccessRule{
		Allow: access.RuleSet{Users: map[string]struct{}{"bob": {}}},
	}
	if err = store.SaveAccessRule("src", "/Old/", rule); err != nil {
		t.Fatal(err)
	}

	newRules := access.RuleMap{
		"/old/": rule,
	}
	if err = store.MigrateAccessRulePathsLowercase("src", []string{"/Old/"}, newRules); err != nil {
		t.Fatal(err)
	}

	rules, err := store.GetAccessRulesBySource("src")
	if err != nil {
		t.Fatal(err)
	}
	if rules["/old/"] == nil {
		t.Fatal("expected lowercased rule row")
	}
	if rules["/Old/"] != nil {
		t.Fatal("expected old rule path to be deleted")
	}
}
