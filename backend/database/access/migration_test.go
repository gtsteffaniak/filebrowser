package access

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
)

func TestRemoveRuleByPathKey_exactLegacyKey(t *testing.T) {
	s := NewStorage(nil)
	sourcePath := "test_source"
	legacyKey := "/legacy"

	s.AllRules[sourcePath] = RuleMap{
		legacyKey: {
			Allow: RuleSet{Users: StringSet{"alice": {}}},
			Deny:  RuleSet{Users: make(StringSet), Groups: make(StringSet)},
		},
	}

	s.RemoveRuleByPathKey(sourcePath, legacyKey)

	if _, ok := s.AllRules[sourcePath][legacyKey]; ok {
		t.Fatal("legacy key should be removed")
	}
	if _, ok := s.AllRules[sourcePath][utils.AddTrailingSlashIfNotExists(legacyKey)]; ok {
		t.Fatal("normalized key should not remain when only legacy existed")
	}
}
