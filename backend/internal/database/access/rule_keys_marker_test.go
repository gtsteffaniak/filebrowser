package access_test

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

type migrateCallCounter struct {
	access.SQLPersister
	calls int
}

func (m *migrateCallCounter) MigrateAccessRulePathsLowercase(source string, oldPaths []string, newRules access.RuleMap) error {
	m.calls++
	return m.SQLPersister.MigrateAccessRulePathsLowercase(source, oldPaths, newRules)
}

type failAccessRuleMigratePersister struct {
	access.SQLPersister
}

func (failAccessRuleMigratePersister) MigrateAccessRulePathsLowercase(string, []string, access.RuleMap) error {
	return errors.New("simulated migration failure")
}

func TestFreshDBSetsAccessRulesLowercaseMarker(t *testing.T) {
	s, _, sqlStore := createTestStorageWithSQL(t)
	s.SetSQLStore(sqlStore)

	if err := s.StartupMigrateAccessRuleKeys(); err != nil {
		t.Fatal(err)
	}
	ok, err := sqlStore.AccessRulesLowercaseMigrated()
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected migration marker on empty database")
	}
}

func TestUpgradeMigratesAndSetsMarker(t *testing.T) {
	_, _, sqlStore := createTestStorageWithSQL(t)

	rule := &access.AccessRule{
		Deny: access.RuleSet{Users: map[string]struct{}{"alice": {}}},
	}
	if err := sqlStore.SaveAccessRule("/src", "/Private/", rule); err != nil {
		t.Fatal(err)
	}

	s := access.NewStorage(nil)
	rules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	s.AllRules = rules
	counter := &migrateCallCounter{SQLPersister: sqlStore}
	s.SetSQLStore(counter)

	if err = s.StartupMigrateAccessRuleKeys(); err != nil {
		t.Fatal(err)
	}
	if counter.calls != 1 {
		t.Fatalf("expected one SQL migration, got %d", counter.calls)
	}
	ok, err := sqlStore.AccessRulesLowercaseMigrated()
	if err != nil || !ok {
		t.Fatalf("marker should be set after upgrade, ok=%v err=%v", ok, err)
	}
	byPath, err := sqlStore.GetAccessRulesBySource("/src")
	if err != nil {
		t.Fatal(err)
	}
	if byPath["/private/"] == nil {
		t.Fatal("expected lowercased rule row")
	}
}

func TestSecondStartupSkipsAccessRulesMigration(t *testing.T) {
	_, _, sqlStore := createTestStorageWithSQL(t)
	if err := sqlStore.SaveAccessRule("/src", "/Private/", &access.AccessRule{
		Deny: access.RuleSet{Users: map[string]struct{}{"u": {}}},
	}); err != nil {
		t.Fatal(err)
	}

	s := access.NewStorage(nil)
	rules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	s.AllRules = rules
	counter := &migrateCallCounter{SQLPersister: sqlStore}
	s.SetSQLStore(counter)

	if err = s.StartupMigrateAccessRuleKeys(); err != nil {
		t.Fatal(err)
	}
	if counter.calls != 1 {
		t.Fatalf("first startup: expected 1 migrate call, got %d", counter.calls)
	}

	already, err := sqlStore.AccessRulesLowercaseMigrated()
	if err != nil || !already {
		t.Fatal("marker should be set after first startup")
	}

	s2 := access.NewStorage(nil)
	rules2, err := sqlStore.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	s2.AllRules = rules2
	s2.SetSQLStore(counter)
	if err = s2.StartupMigrateAccessRuleKeys(); err != nil {
		t.Fatal(err)
	}
	if counter.calls != 1 {
		t.Fatalf("second startup must not invoke SQL migration, got %d calls", counter.calls)
	}
}

func TestMigrationFailureDoesNotSetMarker(t *testing.T) {
	_, _, sqlStore := createTestStorageWithSQL(t)
	if err := sqlStore.SaveAccessRule("/src", "/Private/", &access.AccessRule{
		Deny: access.RuleSet{Users: map[string]struct{}{"u": {}}},
	}); err != nil {
		t.Fatal(err)
	}

	s := access.NewStorage(nil)
	rules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	s.AllRules = rules
	s.SetSQLStore(failAccessRuleMigratePersister{SQLPersister: sqlStore})

	if err = s.StartupMigrateAccessRuleKeys(); err == nil {
		t.Fatal("expected migration failure")
	}
	ok, err := sqlStore.AccessRulesLowercaseMigrated()
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("marker must not be set when migration fails")
	}
}
