package access_test

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

func TestStartupMigrationMergesCollidingRules(t *testing.T) {
	store, userStore, sqlStore := createTestStorageWithSQL(t)
	_ = store
	_ = userStore

	ruleA := &access.AccessRule{
		Deny: access.RuleSet{Users: map[string]struct{}{"alice": {}}},
	}
	ruleB := &access.AccessRule{
		Deny: access.RuleSet{Users: map[string]struct{}{"bob": {}}},
	}
	if err := sqlStore.SaveAccessRule("/src", "/Private/", ruleA); err != nil {
		t.Fatal(err)
	}
	if err := sqlStore.SaveAccessRule("/src", "/private/", ruleB); err != nil {
		t.Fatal(err)
	}

	s := access.NewStorage(nil)
	rules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	s.AllRules = rules
	s.SetSQLStore(sqlStore)

	if startErr := s.StartupMigrateAccessRuleKeys(); startErr != nil {
		t.Fatal(startErr)
	}
	byPath, err := sqlStore.GetAccessRulesBySource("/src")
	if err != nil {
		t.Fatal(err)
	}
	if len(byPath) != 1 {
		t.Fatalf("expected one merged rule row, got %d", len(byPath))
	}
	merged := byPath["/private/"]
	if merged == nil {
		t.Fatal("expected merged rule at lowercase key")
	}
	if _, ok := merged.Deny.Users["alice"]; !ok {
		t.Fatal("missing alice from merge")
	}
	if _, ok := merged.Deny.Users["bob"]; !ok {
		t.Fatal("missing bob from merge")
	}

	migrated, err := sqlStore.AccessRulesLowercaseMigrated()
	if err != nil || !migrated {
		t.Fatal("marker should be set after first migration")
	}
	byPath2, err := sqlStore.GetAccessRulesBySource("/src")
	if err != nil {
		t.Fatal(err)
	}
	if len(byPath2) != 1 {
		t.Fatal("rules should remain after marker is set without re-running migration")
	}
}
