package indexing

import (
	"path/filepath"
	"runtime"
	"strings"
)

// stripFinalPathPrefix turns a Windows GetFinalPathNameByHandle
// VOLUME_NAME_DOS result into a normal path.
func stripFinalPathPrefix(p string) string {
	if strings.HasPrefix(p, `\\?\UNC\`) {
		return `\\` + strings.TrimPrefix(p, `\\?\UNC\`)
	}
	if strings.HasPrefix(p, `\\?\`) {
		return strings.TrimPrefix(p, `\\?\`)
	}
	return p
}

// targetWithinRoot reports whether targetReal is equal to or nested under
// rootReal. Both inputs must already be resolved.
func targetWithinRoot(rootReal, targetReal string) bool {
	if runtime.GOOS == "windows" {
		rootReal = strings.ToLower(rootReal)
		targetReal = strings.ToLower(targetReal)
	}
	rel, err := filepath.Rel(rootReal, targetReal)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
