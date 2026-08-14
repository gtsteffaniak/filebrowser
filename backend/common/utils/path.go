package utils

import (
	"path"
	"path/filepath"
	"strings"
)

// JoinScopedIndexPath joins a user's index scope with a sanitized path segment.
// Unlike filepath.Join, a leading slash on rel does not discard scope (avoids absolute-path scope bypass).
func JoinScopedIndexPath(scope, rel string) string {
	rel = strings.TrimPrefix(path.Clean(rel), "/")
	scope = strings.TrimRight(scope, "/")
	if rel == "" {
		if scope == "" || scope == "/" {
			return "/"
		}
		return scope
	}
	if scope == "" || scope == "/" {
		return "/" + rel
	}
	return scope + "/" + rel
}

// JoinUnderSourceRoot maps an index path onto a source filesystem root without treating
// indexPath as a host-absolute path (filepath.Join would discard sourceRoot when indexPath starts with "/").
func JoinUnderSourceRoot(sourceRoot, indexPath string) string {
	rel := strings.TrimPrefix(path.Clean(indexPath), "/")
	if rel == "" {
		return sourceRoot
	}
	return filepath.Join(sourceRoot, rel)
}
