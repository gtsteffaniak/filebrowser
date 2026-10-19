package access

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestPermittedDenyCaseVariant(t *testing.T) {
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	if err := s.DenyUser("/data", utils.IndexPathFromNormalized("/private/secret.txt", false), "alice"); err != nil {
		t.Fatal(err)
	}
	key := s.ruleKeyFor("/data", utils.IndexPathFromNormalized("/private/secret.txt", false))
	if key != "/private/secret.txt/" {
		t.Fatalf("stored key %q", key)
	}
	if s.Permitted("/data", utils.IndexPathFromNormalized("/PRIVATE/secret.txt", false), "alice") {
		t.Fatal("case variant must hit deny rule")
	}
}

func TestRemoveRuleByPathKeyCaseVariant(t *testing.T) {
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	if err := s.DenyUser("/data", utils.IndexPathFromNormalized("/private/x", false), "alice"); err != nil {
		t.Fatal(err)
	}
	s.RemoveRuleByPathKey("/data", "/PRIVATE/x/")
	if !s.Permitted("/data", utils.IndexPathFromNormalized("/private/x", false), "alice") {
		t.Fatal("rule should be removed when key casing differs")
	}
}

func TestPermittedHotPathNoFilesystemAccess(t *testing.T) {
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	_ = s.DenyUser("/data", utils.IndexPathFromNormalized("/a/b", false), "u")
	if !s.Permitted("/data", utils.IndexPathFromNormalized("/A/B", false), "other") {
		t.Fatal("expected permitted for non-denied user")
	}
}

func BenchmarkPermittedLowercaseNoFS(b *testing.B) {
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	_ = s.DenyUser("/data", utils.IndexPathFromNormalized("/x/y", false), "u")
	path := utils.IndexPathFromNormalized("/X/Y", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Permitted("/data", path, "bob")
	}
}

func TestRulesNeedLowercaseMigration(t *testing.T) {
	if rulesNeedLowercaseMigration(RuleMap{"/ok/": {}}) {
		t.Fatal("already lowercase")
	}
	if !rulesNeedLowercaseMigration(RuleMap{"/Private/": {}}) {
		t.Fatal("uppercase segment should need migration")
	}
	if !rulesNeedLowercaseMigration(RuleMap{"/Private/": {}, "/private/": {}}) {
		t.Fatal("case collision should need migration")
	}
}
