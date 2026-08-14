package utils

import (
	"path"
	"path/filepath"
	"strings"
)

// CleanIndexPathSegment normalizes a path segment as if it were rooted, so relative
// ".." components cannot escape via path.Clean on an unrooted input.
func CleanIndexPathSegment(segment string) string {
	return strings.TrimPrefix(path.Clean("/"+segment), "/")
}

// JoinIndexPathComponents combines index path segments into a single rooted Unix path.
func JoinIndexPathComponents(relativePath ...string) string {
	if len(relativePath) == 0 {
		return "/"
	}
	cleaned := CleanIndexPathSegment(relativePath[0])
	for _, p := range relativePath[1:] {
		segment := CleanIndexPathSegment(p)
		if segment == "" {
			continue
		}
		if cleaned == "" {
			cleaned = segment
			continue
		}
		cleaned = cleaned + "/" + segment
	}
	if cleaned == "" {
		return "/"
	}
	return "/" + cleaned
}

// JoinScopedIndexPath joins a user's index scope with a sanitized path segment.
// Unlike filepath.Join, a leading slash on rel does not discard scope (avoids absolute-path scope bypass).
func JoinScopedIndexPath(scope, rel string) string {
	rel = CleanIndexPathSegment(rel)
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
	rel := CleanIndexPathSegment(indexPath)
	if rel == "" {
		return sourceRoot
	}
	return filepath.Join(sourceRoot, rel)
}
