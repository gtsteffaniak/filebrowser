package access

import (
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Access rules are stored and matched using lowercase path keys (ASCII ToLower
// only; not full Unicode case folding).

// ruleKeyForNL returns the storage key; caller must hold s.mux.
func (s *Storage) ruleKeyForNL(_ string, indexPath utils.IndexPath) string {
	return strings.ToLower(ruleKey(indexPath))
}

// ruleKeyFor returns the storage/lookup key for a rule path on this source.
func (s *Storage) ruleKeyFor(sourcePath string, indexPath utils.IndexPath) string {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.ruleKeyForNL(sourcePath, indexPath)
}

func (s *Storage) checkPathForSourceNL(_ string, indexPath utils.IndexPath) utils.IndexPath {
	p := checkPath(indexPath)
	return utils.IndexPathFromNormalized(strings.ToLower(p.String()), p.IsDir)
}

// checkPathForSource normalizes an index path for permission checks on a source.
func (s *Storage) checkPathForSource(sourcePath string, indexPath utils.IndexPath) utils.IndexPath {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.checkPathForSourceNL(sourcePath, indexPath)
}

// StartupMigrateAccessRuleKeys lowercases existing rule keys in memory and SQL,
// merging colliding paths. Idempotent when keys are already lowercase.
func (s *Storage) StartupMigrateAccessRuleKeys() error {
	s.mux.Lock()
	defer s.mux.Unlock()

	for sourcePath := range s.AllRules {
		if err := s.migrateSourceRulesLowercaseNL(sourcePath); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) migrateSourceRulesLowercaseNL(sourcePath string) error {
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
		newRules[lower] = cloneAccessRule(rule)
		if lower != path {
			logger.Infof("access rules: migrated rule path %q -> %q on source %s", path, lower, sourcePath)
		}
	}

	if !rulesNeedLowercaseMigration(rulesBySource) {
		return nil
	}

	s.AllRules[sourcePath] = newRules

	if s.sqlStore == nil {
		return nil
	}
	return s.sqlStore.MigrateAccessRulePathsLowercase(sourcePath, oldPaths, newRules)
}

func rulesNeedLowercaseMigration(rules RuleMap) bool {
	byLower := make(map[string]string)
	for path := range rules {
		lower := strings.ToLower(path)
		if path != lower {
			return true
		}
		if other, ok := byLower[lower]; ok && other != path {
			return true
		}
		byLower[lower] = path
	}
	return false
}

func mergeAccessRulesInPlace(dst, src *AccessRule) {
	if src == nil || dst == nil {
		return
	}
	if src.DenyAll {
		dst.DenyAll = true
	}
	if dst.Deny.Users == nil {
		dst.Deny.Users = StringSet{}
	}
	if dst.Deny.Groups == nil {
		dst.Deny.Groups = StringSet{}
	}
	if dst.Allow.Users == nil {
		dst.Allow.Users = StringSet{}
	}
	if dst.Allow.Groups == nil {
		dst.Allow.Groups = StringSet{}
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
