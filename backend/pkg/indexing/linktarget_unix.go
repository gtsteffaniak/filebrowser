//go:build !windows

package indexing

import "path/filepath"

func resolveLinkTarget(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
