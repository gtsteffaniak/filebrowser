package fileutils

import (
	"os"
	"path/filepath"
)

// Copy copies a file or folder from one place to another.
func CopyHelper(src, dst string) error {
	src = filepath.Clean(src)
	if src == "" {
		return os.ErrNotExist
	}

	dst = filepath.Clean(dst)
	if dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the root directory.
		return os.ErrInvalid
	}

	if dst == src {
		return os.ErrInvalid
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return CopyDir(src, dst)
	}

	return CopyFile(src, dst)
}
