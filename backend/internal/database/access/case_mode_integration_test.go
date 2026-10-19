package access_test

import (
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

type integStubInfo struct {
	name string
	mode fs.FileMode
}

func (s integStubInfo) Name() string       { return s.name }
func (s integStubInfo) Size() int64        { return 0 }
func (s integStubInfo) Mode() fs.FileMode  { return s.mode }
func (s integStubInfo) IsDir() bool        { return s.mode.IsDir() }
func (s integStubInfo) ModTime() time.Time { return time.Time{} }
func (s integStubInfo) Sys() interface{}   { return nil }

type integDirEntry struct{ name string }

func (e integDirEntry) Name() string               { return e.name }
func (e integDirEntry) IsDir() bool                { return true }
func (e integDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e integDirEntry) Info() (fs.FileInfo, error) { return integStubInfo{name: e.name, mode: fs.ModeDir}, nil }

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

	root := t.TempDir()
	access.SetSourceFSForTest(&access.TestAccessFS{
		ReadDirFn: func(name string) ([]fs.DirEntry, error) {
			if name == root {
				return []fs.DirEntry{integDirEntry{name: "Folder"}}, nil
			}
			return nil, fs.ErrNotExist
		},
		LstatFn: func(name string) (fs.FileInfo, error) {
			if name == filepath.Join(root, "fOLDER") {
				return integStubInfo{name: "fOLDER", mode: fs.ModeDir}, nil
			}
			return nil, fs.ErrNotExist
		},
	})
	t.Cleanup(func() { access.SetSourceFSForTest(nil) })

	if startErr := s.StartupCaseModes(map[string]string{"/src": root}, sqlStore); startErr != nil {
		t.Fatal(startErr)
	}
	got, ok, err := sqlStore.GetAccessSourceCaseMode("/src")
	if err != nil || !ok || !got {
		t.Fatalf("mode persisted: ok=%v insensitive=%v err=%v", ok, got, err)
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
}
