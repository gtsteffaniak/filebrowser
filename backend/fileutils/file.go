package fileutils

import (
	"io"
	"os"
	"path"
	"path/filepath"
)

// MoveFile moves a file from src to dst.
// By default, the rename system call is used. If src and dst point to different volumes,
// the file copy is used as a fallback.
func MoveFile(src, dst string) error {
	if os.Rename(src, dst) == nil {
		return nil
	}
	// fallback
	err := CopyFile(src, dst)
	if err != nil {
		_ = os.Remove(dst)
		return err
	}
	if err := os.Remove(src); err != nil {
		return err
	}
	return nil
}

// CopyFile copies a file from source to dest and returns
// an error if any.
func CopyFile(source, dest string) error {
	// Open the source file.
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	// Makes the directory needed to create the dst file.
	err = os.MkdirAll(filepath.Dir(dest), 0775) //nolint:gomnd
	if err != nil {
		return err
	}

	// Create the destination file.
	dst, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666) //nolint:gomnd
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy the contents of the file.
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	// Copy the mode.
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	err = os.Chmod(dest, info.Mode())
	if err != nil {
		return err
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
