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

// SetFsPermissions sets create modes from Unix chmod(2)-style values (for example
// values produced by strconv.ParseUint(s, 8, 32)). setuid, setgid, and sticky
// bits must be converted for Go's os.FileMode; a plain cast from Unix octal is
// incorrect for those bits.
func SetFsPermissions(unixFileMode, unixDirMode uint32) {
	PermFile = unixModeToFileMode(unixFileMode)
	PermDir = unixModeToFileMode(unixDirMode)
}

// EffectiveDirPerm returns [PermDir] if [SetFsPermissions] was called, otherwise 0o755. On Unix,
// [os.Mkdir] with perm 0 fails with permission denied, so use this (or [PermFile]) for creates when
// the process has not set globals yet.
func EffectiveDirPerm() os.FileMode {
	if PermDir == 0 {
		return 0o755
	}
	return PermDir
}

// EffectiveFilePerm returns [PermFile] if set, otherwise 0o644.
func EffectiveFilePerm() os.FileMode {
	if PermFile == 0 {
		return 0o644
	}
	return PermFile
}

func unixModeToFileMode(u uint32) os.FileMode {
	m := os.FileMode(u & 0777)
	if u&0o4000 != 0 {
		m |= os.ModeSetuid
	}
	if u&0o2000 != 0 {
		m |= os.ModeSetgid
	}
	if u&0o1000 != 0 {
		m |= os.ModeSticky
	}
	return m
}

// MoveFile moves a file from src to dst.
// By default, the rename system call is used. If src and dst are on different volumes,
// CopyFile is used as a fallback and the source is removed synchronously on success.
func MoveFile(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		if !isCrossDeviceRenameError(err) {
			return err
		}
		if err := CopyFile(src, dst); err != nil {
			return err
		}
		return os.RemoveAll(src)
	}
	return nil
}

func preserveFileMode(path string, mode os.FileMode) error {
	return os.Chmod(path, mode.Perm())
}

// CopyFile copies a file or directory from source to dest and returns an error if any.
func CopyFile(source, dest string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return copySymlink(source, dest)
	}
	if info.IsDir() {
		return copyDirectory(source, dest)
	}
	return copySingleFile(source, dest)
}

func copySymlink(source, dest string) error {
	target, err := os.Readlink(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), EffectiveDirPerm()); err != nil {
		return err
	}
	return os.Symlink(target, dest)
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
	err = os.MkdirAll(filepath.Dir(dest), EffectiveDirPerm())
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
	// Preserve source file mod time
	if err := os.Chtimes(dest, srcInfo.ModTime(), srcInfo.ModTime()); err != nil {
		logger.Debugf("Could not preserve modification time for %s: %v", dest, err)
	}
	return nil
}

// copyDirectory handles copying directories recursively.
func copyDirectory(source, dest string) error {
	srcInfo, err := os.Lstat(source)
	if err != nil {
		return err
	}
	// Create the destination directory.
	err = os.MkdirAll(dest, EffectiveDirPerm())
	if err != nil {
		return err
	}
	if err = preserveFileMode(dest, srcInfo.Mode()); err != nil {
		logger.Debugf("Could not set directory permissions for %s: %v", dest, err)
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

		if entry.Type()&os.ModeSymlink != 0 {
			linkTarget := ""
			linkTarget, err = os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err = os.Symlink(linkTarget, destPath); err != nil {
				return err
			}
			continue
		}
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
	return os.Chtimes(dest, srcInfo.ModTime(), srcInfo.ModTime())
}

// PreserveModTimes copies the mod times from src onto dst, this is used by the webdav COPY handler
func PreserveModTimes(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return os.Chtimes(dst, info.ModTime(), info.ModTime())
	}
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			logger.Debugf("Error accessing %s to set mod time: %v", p, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			logger.Debugf("Error getting info for %s: %v", p, err)
			return nil
		}
		if err := os.Chtimes(filepath.Join(dst, rel), info.ModTime(), info.ModTime()); err != nil {
			logger.Debugf("Could not preserve modification time for %s: %v", filepath.Join(dst, rel), err)
		}
		return nil
	})
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
		path := filepath.Join(cacheDir, entry.Name())
		err = os.RemoveAll(path)
		if err != nil {
			logger.Errorf("failed clear cache dir: %v", err)
		}
	}

}

// ClearDirectoryContents removes all files and subdirectories inside dir; dir itself remains.
// If dir does not exist, returns nil.
func ClearDirectoryContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}
