package access

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Access rules on case-insensitive filesystems are stored and matched using
// lowercase path keys (ASCII ToLower only; not full Unicode case folding).

// CaseModeStore persists per-source case-insensitivity mode.
type CaseModeStore interface {
	GetAccessSourceCaseMode(source string) (insensitive bool, found bool, err error)
	SaveAccessSourceCaseMode(source string, insensitive bool) error
}

type accessFS interface {
	ReadDir(name string) ([]fs.DirEntry, error)
	Lstat(name string) (fs.FileInfo, error)
}

type osAccessFS struct{}

func (osAccessFS) ReadDir(name string) ([]fs.DirEntry, error) { return os.ReadDir(name) }
func (osAccessFS) Lstat(name string) (fs.FileInfo, error)   { return os.Lstat(name) }

var sourceFS accessFS = osAccessFS{}

// TestAccessFS implements accessFS for tests.
type TestAccessFS struct {
	ReadDirFn func(string) ([]fs.DirEntry, error)
	LstatFn   func(string) (fs.FileInfo, error)
}

func (t *TestAccessFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if t.ReadDirFn != nil {
		return t.ReadDirFn(name)
	}
	return nil, fs.ErrNotExist
}

func (t *TestAccessFS) Lstat(name string) (fs.FileInfo, error) {
	if t.LstatFn != nil {
		return t.LstatFn(name)
	}
	return nil, fs.ErrNotExist
}

// SetSourceFSForTest overrides filesystem probes; pass nil to restore defaults.
func SetSourceFSForTest(fs *TestAccessFS) {
	if fs == nil {
		sourceFS = osAccessFS{}
		return
	}
	sourceFS = fs
}

func platformDefaultCaseInsensitive() bool {
	switch runtime.GOOS {
	case "windows", "darwin":
		return true
	default:
		return false
	}
}

// DetectCaseInsensitiveSource reports whether paths on sourceRoot are matched
// case-insensitively. Uses the platform default, refined by a read-only check:
// one ReadDir of the source root and Lstat of an existing entry under a
// case-flipped ASCII name. No files are created.
func DetectCaseInsensitiveSource(sourceRoot string) bool {
	return detectCaseInsensitiveSource(sourceFS, sourceRoot)
}

func detectCaseInsensitiveSource(fs accessFS, sourceRoot string) bool {
	def := platformDefaultCaseInsensitive()
	entries, err := fs.ReadDir(sourceRoot)
	if err != nil {
		return def
	}
	for _, e := range entries {
		name := e.Name()
		flipped := flipASCIICase(name)
		if flipped == name {
			continue
		}
		if _, err := fs.Lstat(filepath.Join(sourceRoot, flipped)); err == nil {
			return true
		}
	}
	return def
}

func flipASCIICase(s string) string {
	var changed bool
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			b.WriteByte(byte(r + ('a' - 'A')))
			changed = true
			continue
		}
		if r >= 'a' && r <= 'z' {
			b.WriteByte(byte(r - ('a' - 'A')))
			changed = true
			continue
		}
		b.WriteRune(r)
	}
	if !changed {
		return s
	}
	return b.String()
}

func (s *Storage) caseInsensitiveSourceNL(sourcePath string) bool {
	return s.sourceCaseInsensitive != nil && s.sourceCaseInsensitive[sourcePath]
}

// ruleKeyForNL returns the storage key; caller must hold s.mux.
func (s *Storage) ruleKeyForNL(sourcePath string, indexPath utils.IndexPath) string {
	key := ruleKey(indexPath)
	if s.caseInsensitiveSourceNL(sourcePath) {
		return strings.ToLower(key)
	}
	return key
}

// ruleKeyFor returns the storage/lookup key for a rule path on this source.
func (s *Storage) ruleKeyFor(sourcePath string, indexPath utils.IndexPath) string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.ruleKeyForNL(sourcePath, indexPath)
}

func (s *Storage) checkPathForSourceNL(sourcePath string, indexPath utils.IndexPath) utils.IndexPath {
	p := checkPath(indexPath)
	if !s.caseInsensitiveSourceNL(sourcePath) {
		return p
	}
	return utils.IndexPathFromNormalized(strings.ToLower(p.String()), p.IsDir)
}

// checkPathForSource normalizes an index path for permission checks on a source.
func (s *Storage) checkPathForSource(sourcePath string, indexPath utils.IndexPath) utils.IndexPath {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.checkPathForSourceNL(sourcePath, indexPath)
}

// StartupCaseModes detects case mode per source, migrates rules when needed, and persists mode.
// sourceRoots maps access source key (AllRules key) to filesystem root path.
func (s *Storage) StartupCaseModes(sourceRoots map[string]string, store CaseModeStore) error {
	if store == nil {
		return nil
	}
	s.mux.Lock()
	defer s.mux.Unlock()

	if s.sourceCaseInsensitive == nil {
		s.sourceCaseInsensitive = make(map[string]bool)
	}

	sources := make(map[string]struct{})
	for sp := range sourceRoots {
		sources[sp] = struct{}{}
	}
	for sp := range s.AllRules {
		sources[sp] = struct{}{}
	}

	for sourcePath := range sources {
		root := sourceRoots[sourcePath]
		if root == "" {
			root = sourcePath
		}
		detected := detectCaseInsensitiveSource(sourceFS, root)

		saved, hasSaved, err := store.GetAccessSourceCaseMode(sourcePath)
		if err != nil {
			return err
		}

		if hasSaved && saved && !detected {
			logger.Warningf("access rules: source %q detected case-sensitive filesystem but was case-insensitive; rule keys left unchanged", sourcePath)
		}

		needMigrate := detected && (!hasSaved || !saved)
		if needMigrate {
			if err := s.migrateSourceRulesInsensitiveNL(sourcePath); err != nil {
				return err
			}
		}

		s.sourceCaseInsensitive[sourcePath] = detected
		if err := store.SaveAccessSourceCaseMode(sourcePath, detected); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) migrateSourceRulesInsensitiveNL(sourcePath string) error {
	rulesBySource, ok := s.AllRules[sourcePath]
	if !ok || len(rulesBySource) == 0 {
		return nil
	}

	oldPaths := make([]string, 0, len(rulesBySource))
	newRules := make(RuleMap)
	for path, rule := range rulesBySource {
		oldPaths = append(oldPaths, path)
		lower := strings.ToLower(path)
		if existing, exists := newRules[lower]; exists {
			logger.Infof("access rules: merging colliding paths %q into %q on source %s", path, lower, sourcePath)
			mergeAccessRulesInPlace(existing, rule)
			continue
		}
		newRules[lower] = cloneAccessRuleForMigrate(rule)
		if lower != path {
			logger.Infof("access rules: migrated rule path %q -> %q on source %s", path, lower, sourcePath)
		}
	}

	s.AllRules[sourcePath] = newRules

	if s.sqlStore == nil {
		return nil
	}
	for _, old := range oldPaths {
		if err := s.sqlStore.DeleteAccessRule(sourcePath, old); err != nil {
			return err
		}
	}
	for path, rule := range newRules {
		if err := s.sqlStore.SaveAccessRule(sourcePath, path, rule); err != nil {
			return err
		}
	}
	return nil
}

func cloneAccessRuleForMigrate(rule *AccessRule) *AccessRule {
	if rule == nil {
		return nil
	}
	cp := func(src StringSet) StringSet {
		if len(src) == 0 {
			return nil
		}
		dst := make(StringSet, len(src))
		for k := range src {
			dst[k] = struct{}{}
		}
		return dst
	}
	return &AccessRule{
		DenyAll: rule.DenyAll,
		Deny:    RuleSet{Users: cp(rule.Deny.Users), Groups: cp(rule.Deny.Groups)},
		Allow:   RuleSet{Users: cp(rule.Allow.Users), Groups: cp(rule.Allow.Groups)},
	}
}

func mergeAccessRulesInPlace(dst, src *AccessRule) {
	if src == nil || dst == nil {
		return
	}
	if src.DenyAll {
		dst.DenyAll = true
	}
	for u := range src.Deny.Users {
		dst.Deny.Users[u] = struct{}{}
	}
	for g := range src.Deny.Groups {
		dst.Deny.Groups[g] = struct{}{}
	}
	for u := range src.Allow.Users {
		dst.Allow.Users[u] = struct{}{}
	}
	for g := range src.Allow.Groups {
		dst.Allow.Groups[g] = struct{}{}
	}
}

func (s *Storage) deleteStoredRuleNL(sourcePath, rulePath string) {
	delete(s.AllRules[sourcePath], rulePath)
	if len(s.AllRules[sourcePath]) == 0 {
		delete(s.AllRules, sourcePath)
	}
}
