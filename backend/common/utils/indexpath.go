package utils

import (
	"fmt"
	"strings"
)

// IndexPath is a normalized index-relative path with explicit directory vs file semantics.
// Parts holds path segments only (no empty strings). An empty Parts slice represents root "/".
// String() always returns a path starting with "/". Directories include a trailing slash; files do not.
type IndexPath struct {
	Parts []string
	IsDir bool
}

// NewIndexPath builds an IndexPath from already-separated segments (internal use).
func NewIndexPath(parts []string, isDir bool) IndexPath {
	return IndexPath{Parts: parts, IsDir: isDir}
}

// IndexPathFromNormalized parses an already-normalized slash-separated path (internal/indexer use).
func IndexPathFromNormalized(path string, isDir bool) IndexPath {
	p, _ := parseIndexPath(path, isDir, false)
	return p
}

// ParseSanitizedIndexPath validates external path input then parses a slash-separated index path.
func ParseSanitizedIndexPath(userPath string, isDir bool) (IndexPath, error) {
	clean, err := SanitizePath(userPath)
	if err != nil {
		return IndexPath{}, err
	}
	return parseIndexPath(clean, isDir, true)
}

func parseIndexPath(path string, isDir bool, strict bool) (IndexPath, error) {
	if strict && strings.Contains(path, `\`) {
		return IndexPath{}, fmt.Errorf("invalid path: backslashes not allowed")
	}

	inner := strings.Trim(path, "/")
	if inner == "" {
		return IndexPath{IsDir: isDir}, nil
	}

	parts := strings.FieldsFunc(inner, func(r rune) bool {
		return r == '/'
	})
	if strict {
		for _, part := range parts {
			if part == "." || part == ".." {
				return IndexPath{}, fmt.Errorf("invalid path: %q segment not allowed", part)
			}
		}
	}

	return IndexPath{Parts: parts, IsDir: isDir}, nil
}

// String returns the canonical index path string. Never returns an empty string; root is "/".
func (p IndexPath) String() string {
	if len(p.Parts) == 0 {
		return "/"
	}
	s := "/" + strings.Join(p.Parts, "/")
	if p.IsDir {
		return s + "/"
	}
	return s
}

// AsDirectory returns a copy with directory semantics (trailing slash when stringified).
func (p IndexPath) AsDirectory() IndexPath {
	if p.IsDir {
		return p
	}
	return IndexPath{Parts: append([]string(nil), p.Parts...), IsDir: true}
}

// IsRoot reports whether this path is the index root.
func (p IndexPath) IsRoot() bool {
	return len(p.Parts) == 0
}

// Parent returns the parent directory path. Root's parent is root.
func (p IndexPath) Parent() IndexPath {
	if p.IsRoot() {
		return IndexPath{IsDir: true}
	}
	return IndexPath{Parts: append([]string(nil), p.Parts[:len(p.Parts)-1]...), IsDir: true}
}

// Join appends a single path segment and sets directory vs file semantics.
func (p IndexPath) Join(name string, isDir bool) IndexPath {
	return IndexPath{Parts: append(append([]string(nil), p.Parts...), name), IsDir: isDir}
}

// RuleKey returns the canonical string key for access rule storage (always directory form).
func (p IndexPath) RuleKey() string {
	return p.AsDirectory().String()
}
