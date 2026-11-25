package fileutils

import (
	"fmt"
	"os"
	"path/filepath"
)

// MoveFileMerge moves a file or directory from src to dst.
// Existing directories are merged recursively, conflicting files are replaced,
// and destination-only entries are preserved.
func MoveFileMerge(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}

	dstInfo, err := os.Lstat(dst)
	if os.IsNotExist(err) {
		return MoveFile(src, dst)
	}
	if err != nil {
		return err
	}
	if srcInfo.IsDir() != dstInfo.IsDir() {
		return fmt.Errorf("cannot merge %q into %q: source and destination have different types", src, dst)
	}
	if !srcInfo.IsDir() {
		return MoveFile(src, dst)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if err := MoveFileMerge(srcPath, dstPath); err != nil {
			return fmt.Errorf("merge %q: %w", entry.Name(), err)
		}
	}
	return os.Remove(src)
}
