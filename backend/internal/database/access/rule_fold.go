package access

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// foldedRuleKey is the case-folded form of a rule storage path (RuleKey / directory form).
func foldedRuleKey(rulePath string) string {
	return strings.ToLower(rulePath)
}

// RebuildFoldedRuleIndex rebuilds the in-memory case-folded rule path index for all sources.
// Call after bulk-loading rules from SQL.
func (s *Storage) RebuildFoldedRuleIndex() {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.rebuildFoldedRuleIndexNL()
}

func (s *Storage) rebuildFoldedRuleIndexNL() {
	s.foldedRulePaths = make(map[string]map[string]string)
	for sourcePath, rules := range s.AllRules {
		for rulePath := range rules {
			s.setFoldedRuleNL(sourcePath, rulePath)
		}
	}
}

func (s *Storage) setFoldedRuleNL(sourcePath, rulePath string) {
	if s.foldedRulePaths == nil {
		s.foldedRulePaths = make(map[string]map[string]string)
	}
	byFold, ok := s.foldedRulePaths[sourcePath]
	if !ok {
		byFold = make(map[string]string)
		s.foldedRulePaths[sourcePath] = byFold
	}
	byFold[foldedRuleKey(rulePath)] = rulePath
}

func (s *Storage) unsetFoldedRuleNL(sourcePath, rulePath string) {
	byFold, ok := s.foldedRulePaths[sourcePath]
	if !ok {
		return
	}
	delete(byFold, foldedRuleKey(rulePath))
	if len(byFold) == 0 {
		delete(s.foldedRulePaths, sourcePath)
	}
}

func (s *Storage) renameFoldedRuleNL(sourcePath, oldPath, newPath string) {
	s.unsetFoldedRuleNL(sourcePath, oldPath)
	s.setFoldedRuleNL(sourcePath, newPath)
}

func (s *Storage) deleteStoredRuleNL(sourcePath, rulePath string) {
	delete(s.AllRules[sourcePath], rulePath)
	s.unsetFoldedRuleNL(sourcePath, rulePath)
	if len(s.AllRules[sourcePath]) == 0 {
		delete(s.AllRules, sourcePath)
	}
}

// needsCaseFoldResolution reports whether a case-insensitive rule might apply at a
// different spelling than indexPath (or an ancestor) uses.
func (s *Storage) needsCaseFoldResolution(sourcePath string, indexPath utils.IndexPath) bool {
	if !s.isSourceCaseInsensitive(sourcePath) {
		return false
	}
	s.mux.RLock()
	rules := s.AllRules[sourcePath]
	folded := s.foldedRulePaths[sourcePath]
	s.mux.RUnlock()
	if len(rules) == 0 || len(folded) == 0 {
		return false
	}

	current := checkPath(indexPath)
	for {
		exact := ruleKey(current)
		if _, ok := rules[exact]; ok {
			// Exact rule key matches this path spelling.
		} else if _, ok := folded[foldedRuleKey(exact)]; ok {
			return true
		}
		if current.IsRoot() {
			break
		}
		current = current.Parent()
	}
	return false
}

func (s *Storage) isSourceCaseInsensitive(sourcePath string) bool {
	s.mux.RLock()
	if s.caseInsensitive != nil {
		if v, ok := s.caseInsensitive[sourcePath]; ok {
			s.mux.RUnlock()
			return v
		}
	}
	s.mux.RUnlock()

	s.mux.Lock()
	defer s.mux.Unlock()
	if s.caseInsensitive == nil {
		s.caseInsensitive = make(map[string]bool)
	}
	if v, ok := s.caseInsensitive[sourcePath]; ok {
		return v
	}
	v := detectCaseInsensitiveFilesystem(sourcePath)
	s.caseInsensitive[sourcePath] = v
	return v
}

// detectCaseInsensitiveFilesystem probes the source root once with a temporary file.
func detectCaseInsensitiveFilesystem(sourceRoot string) bool {
	probeName := "._fb_case_probe"
	probePath := filepath.Join(sourceRoot, probeName)
	f, err := os.Create(probePath)
	if err != nil {
		return false
	}
	_ = f.Close()
	defer os.Remove(probePath)

	flipped := flipASCIICase(probeName)
	if flipped == probeName {
		return false
	}
	_, err = os.Lstat(filepath.Join(sourceRoot, flipped))
	return err == nil
}

func flipASCIICase(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsUpper(r) {
			b.WriteRune(unicode.ToLower(r))
		} else if unicode.IsLower(r) {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
