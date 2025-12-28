package fileutils

import (
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/gtsteffaniak/go-logger/logger"
)

var PermFile os.FileMode
var PermDir os.FileMode

func SetFsPermissions(PermFileOctal os.FileMode, PermDirOctal os.FileMode) {
	PermFile = PermFileOctal
	PermDir = PermDirOctal
}

// MoveFile moves a file from src to dst.
// By default, the rename system call is used. If src and dst point to different volumes,
// the file copy is used as a fallback.
func MoveFile(src, dst string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	// fallback
	err = CopyFile(src, dst)
	if err != nil {
		logger.Errorf("CopyFile failed %v", err)
		return err
	}

	go func() {
		err = os.RemoveAll(src)
		if err != nil {
			logger.Errorf("os.Remove failed %v", err)
		}
	}()

	return nil
}

// CopyFile copies a file or directory from source to dest and returns an error if any.
func CopyFile(source, dest string) error {
	// Check if the source exists and whether it's a file or directory.
	info, err := os.Stat(source)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// If the source is a directory, copy it recursively.
		return copyDirectory(source, dest)
	}

	// If the source is a file, copy the file.
	return copySingleFile(source, dest)
}

// copySingleFile handles copying a single file.
func copySingleFile(source, dest string) error {
	// Get source file info to preserve permissions
	srcInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	sourcePerms := srcInfo.Mode().Perm()

	// Open the source file.
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	// Create the destination directory if needed.
	err = os.MkdirAll(filepath.Dir(dest), PermDir)
	if err != nil {
		return err
	}

	// Create the destination file with source permissions
	dst, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, sourcePerms)
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy the contents of the file.
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	// Preserve source file permissions
	// Handle chmod errors gracefully (e.g., in rootless containers where chmod may be restricted)
	err = os.Chmod(dest, sourcePerms)
	if err != nil {
		// Log but don't fail - chmod may be restricted in some environments
		// The file was copied successfully, so we continue
		logger.Debugf("Could not set file permissions for %s (this may be expected in restricted environments): %v", dest, err)
	}

	return nil
}

// copyDirectory handles copying directories recursively.
func copyDirectory(source, dest string) error {
	// Create the destination directory.
	err := os.MkdirAll(dest, PermDir)
	if err != nil {
		return err
	}

	// Read the contents of the source directory.
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	// Iterate over each entry in the directory.
	for _, entry := range entries {
		srcPath := filepath.Join(source, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectories.
			err = copyDirectory(srcPath, destPath)
			if err != nil {
				return err
			}
		} else {
			// Copy files.
			err = copySingleFile(srcPath, destPath)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// CommonPrefix returns the common directory path of provided files.
func CommonPrefix(sep byte, paths ...string) string {
	// Handle special cases.
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return path.Clean(paths[0])
	}

	// Treat string as []byte, not []rune as is often done in Go.
	c := []byte(path.Clean(paths[0]))

	// Add a trailing sep to handle the case where the common prefix directory
	// is included in the path list.
	c = append(c, sep)

	// Ignore the first path since it's already in c.
	for _, v := range paths[1:] {
		// Clean up each path before testing it.
		v = path.Clean(v) + string(sep)

		// Find the first non-common byte and truncate c.
		if len(v) < len(c) {
			c = c[:len(v)]
		}
		for i := 0; i < len(c); i++ {
			if v[i] != c[i] {
				c = c[:i]
				break
			}
		}
	}

	// Remove trailing non-separator characters and the final separator.
	for i := len(c) - 1; i >= 0; i-- {
		if c[i] == sep {
			c = c[:i]
			break
		}
	}

	return string(c)
}

func ClearCacheDir(cacheDir string) {
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		logger.Errorf("failed clear cache dir: %v", err)
	}

	for _, entry := range entries {
		// Exclude sql directory - it contains persistent index databases
		if entry.Name() == "sql" {
			continue
		}
		path := filepath.Join(cacheDir, entry.Name())
		err = os.RemoveAll(path)
		if err != nil {
			logger.Errorf("failed clear cache dir: %v", err)
		}
	}

}
