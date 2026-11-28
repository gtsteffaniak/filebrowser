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

// toExtendedLengthPath prefixes Windows paths at or above MAX_PATH with the
// extended-length prefix so Win32 APIs such as FindFirstFile can open them.
func toExtendedLengthPath(p string) string {
	const maxPath = 260
	if len(p) < maxPath || strings.HasPrefix(p, `\\?\`) || strings.HasPrefix(p, `\\.\`) {
		return p
	}
	if strings.HasPrefix(p, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(p, `\\`)
	}
	if ((p[0] >= 'a' && p[0] <= 'z') || (p[0] >= 'A' && p[0] <= 'Z')) && p[1] == ':' && (p[2] == '\\' || p[2] == '/') {
		return `\\?\` + strings.ReplaceAll(p, "/", `\`)
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
