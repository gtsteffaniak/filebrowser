package access_test

import (
	"slices"
	"testing"
)

func TestRenameUserInRules(t *testing.T) {
	setupTestSources()
	s, _, sqlStore := createTestStorageWithSQL(t)
	src := "test_source"

	if err := s.AllowUser(src, idxPath("/allowed"), "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.DenyUser(src, idxPath("/denied"), "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.AllowUser(src, idxPath("/both"), "alice"); err != nil {
		t.Fatal(err)
	}
	if err := s.DenyUser(src, idxPath("/both"), "alice"); err != nil {
		t.Fatal(err)
	}
	// New name already listed on another path (set semantics: old entry removed, new unchanged).
	if err := s.AllowUser(src, idxPath("/existing"), "alicia"); err != nil {
		t.Fatal(err)
	}

	if err := s.RenameUserInRules("alice", "alicia"); err != nil {
		t.Fatal(err)
	}

	if rules := s.GetRulesForUser(src, "alice"); len(rules) != 0 {
		t.Fatalf("expected no rules for old name alice, got %d", len(rules))
	}
	aliciaRules := s.GetRulesForUser(src, "alicia")
	if len(aliciaRules) != 4 {
		t.Fatalf("expected 4 paths for alicia, got %d", len(aliciaRules))
	}
	for _, path := range []string{"/allowed/", "/denied/", "/both/", "/existing/"} {
		if _, ok := aliciaRules[path]; !ok {
			t.Fatalf("missing rule at %q: %v", path, aliciaRules)
		}
	}

	sqlRules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range sqlRules[src] {
		if _, ok := rule.Allow.Users["alice"]; ok {
			t.Fatal("alice should not appear in allow lists in SQL")
		}
		if _, ok := rule.Deny.Users["alice"]; ok {
			t.Fatal("alice should not appear in deny lists in SQL")
		}
	}

	if err := s.RenameUserInRules("nobody", "other"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameUserInRules("alicia", "alicia"); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveAllRulesForUserOnDelete(t *testing.T) {
	setupTestSources()
	s, _, sqlStore := createTestStorageWithSQL(t)
	src := "test_source"

	if err := s.AllowUser(src, idxPath("/a"), "bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.DenyUser(src, idxPath("/b"), "bob"); err != nil {
		t.Fatal(err)
	}

	if err := s.RemoveAllRulesForUser("bob"); err != nil {
		t.Fatal(err)
	}
	if rules := s.GetRulesForUser(src, "bob"); len(rules) != 0 {
		t.Fatalf("expected no in-memory rules for bob, got %d", len(rules))
	}
	all, err := s.GetAllRules(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("expected no rules left on source, got %d", len(all))
	}

	sqlRules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		t.Fatal(err)
	}
	if len(sqlRules[src]) != 0 {
		t.Fatalf("expected no SQL rules for source, got %v", sqlRules[src])
	}
}

func TestRenameUserInRulesBothAllowAndDenySamePath(t *testing.T) {
	setupTestSources()
	s, _, _ := createTestStorageWithSQL(t)
	src := "test_source"

	if err := s.AllowUser(src, idxPath("/shared"), "old"); err != nil {
		t.Fatal(err)
	}
	if err := s.DenyUser(src, idxPath("/shared"), "old"); err != nil {
		t.Fatal(err)
	}

	if err := s.RenameUserInRules("old", "new"); err != nil {
		t.Fatal(err)
	}

	rules, ok := s.GetFrontendRules(src, idxPath("/shared"))
	if !ok {
		t.Fatal("expected rule at /shared")
	}
	if !slices.Contains(rules.Allow.Users, "new") || slices.Contains(rules.Allow.Users, "old") {
		t.Fatalf("allow users: %v", rules.Allow.Users)
	}
	if !slices.Contains(rules.Deny.Users, "new") || slices.Contains(rules.Deny.Users, "old") {
		t.Fatalf("deny users: %v", rules.Deny.Users)
	}
}
