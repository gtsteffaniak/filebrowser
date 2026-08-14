package utils

import (
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func GetParentDirectoryPath(path string) string {
	if path == "/" || path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/") // Remove trailing slash if any
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return "" // No parent directory for a relative path without slashes
	}
	if lastSlash == 0 {
		return "/" // If the last slash is the first character, return root
	}
	return path[:lastSlash]
}

func GetLastComponent(path string) string {
	if path == "" {
		return ""
	}
	path = strings.TrimSuffix(path, "/") // Remove trailing slash if any
	lastSlash := strings.LastIndex(path, "/")
	if lastSlash == -1 {
		return path // No parent directory for a relative path without slashes
	}
	return path[lastSlash+1:]
}

func JoinPathAsUnix(parts ...string) string {
	joinedPath := filepath.Join(parts...)
	if runtime.GOOS == "windows" {
		joinedPath = strings.ReplaceAll(joinedPath, "\\", "/")
	}
	return joinedPath
}

// CleanIndexPathSegment normalizes an index path segment and clamps ".." at the virtual root.
func CleanIndexPathSegment(segment string) string {
	return strings.TrimPrefix(path.Clean("/"+segment), "/")
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

// AddTrailingSlashIfNotExists ensures a directory index path has a trailing slash (root stays "/").
func AddTrailingSlashIfNotExists(indexPath string) string {
	if indexPath == "" || indexPath == "/" {
		return "/"
	}
	if indexPath[len(indexPath)-1] != '/' {
		return indexPath + "/"
	}
	return indexPath
}
