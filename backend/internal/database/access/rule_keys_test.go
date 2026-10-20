package access

import (
	"os"
	"path/filepath"
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

func TestCheckRulePathExistsMixedCase(t *testing.T) {
	root := t.TempDir()
	mixedDir := filepath.Join(root, "MyFolder", "SubDir")
	if err := os.MkdirAll(mixedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mixedFile := filepath.Join(mixedDir, "Readme.MD")
	if err := os.WriteFile(mixedFile, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				root: {Path: root, Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	if err := s.DenyUser(root, utils.IndexPathFromNormalized("/MyFolder/SubDir/Readme.MD", false), "alice"); err != nil {
		t.Fatal(err)
	}

	all, err := s.GetAllRules(root)
	if err != nil {
		t.Fatal(err)
	}
	rule, ok := all["/myfolder/subdir/readme.md/"]
	if !ok {
		t.Fatalf("expected lowercased rule key in GetAllRules, got %v", all)
	}
	if !rule.PathExists {
		t.Fatal("expected PathExists=true for mixed-case path on disk")
	}

	feRule, ok := s.GetFrontendRules(root, utils.IndexPathFromNormalized("/myfolder/subdir/readme.md", false))
	if !ok {
		t.Fatal("expected rule from GetFrontendRules with lowercase query")
	}
	if !feRule.PathExists {
		t.Fatal("expected PathExists=true in GetFrontendRules")
	}

	if checkRulePathExists(root, "/myfolder/subdir/nonexistent.txt/") {
		t.Fatal("expected false for missing path")
	}
}

func TestUpdateRulesAndUpdateRulePathCaseOnlyAndCollision(t *testing.T) {
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	if err := s.DenyUser("/data", utils.IndexPathFromNormalized("/MyFolder", true), "alice"); err != nil {
		t.Fatal(err)
	}

	// Case-only rename is a no-op and preserves the rule.
	updated, err := s.UpdateRules("/data", utils.IndexPathFromNormalized("/MyFolder", true), utils.IndexPathFromNormalized("/myfolder", true))
	if err != nil {
		t.Fatal(err)
	}
	if updated != 0 {
		t.Fatalf("expected 0 updated for case-only rename, got %d", updated)
	}
	if s.Permitted("/data", utils.IndexPathFromNormalized("/MyFolder", true), "alice") {
		t.Fatal("rule should still deny alice after case-only rename")
	}

	if err := s.UpdateRulePath("/data", utils.IndexPathFromNormalized("/MyFolder", true), utils.IndexPathFromNormalized("/myfolder", true)); err != nil {
		t.Fatal(err)
	}
	if s.Permitted("/data", utils.IndexPathFromNormalized("/myfolder", true), "alice") {
		t.Fatal("rule should still deny alice after case-only UpdateRulePath")
	}

	// Colliding target rule merges entries instead of overwriting.
	if err := s.DenyUser("/data", utils.IndexPathFromNormalized("/Archive", true), "bob"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateRulePath("/data", utils.IndexPathFromNormalized("/MyFolder", true), utils.IndexPathFromNormalized("/archive", true)); err != nil {
		t.Fatal(err)
	}
	if s.Permitted("/data", utils.IndexPathFromNormalized("/archive", true), "alice") ||
		s.Permitted("/data", utils.IndexPathFromNormalized("/archive", true), "bob") {
		t.Fatal("expected merged rule at /archive/ to deny both alice and bob")
	}
}

func TestRemoveAllowUserPreservesDenyAll(t *testing.T) {
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	p := utils.IndexPathFromNormalized("/Secret", true)
	if err := s.DenyAll("/data", p); err != nil {
		t.Fatal(err)
	}
	if err := s.AllowUser("/data", p, "alice"); err != nil {
		t.Fatal(err)
	}
	removed, err := s.RemoveAllowUser("/data", p, "alice")
	if err != nil || !removed {
		t.Fatalf("RemoveAllowUser failed: removed=%v err=%v", removed, err)
	}
	if s.Permitted("/data", p, "bob") {
		t.Fatal("DenyAll must remain active after removing the last allowed user")
	}
}
