//go:build !linux

package indexing

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// readDirFileInfos lists dir as FileInfos. Each entry is stat'ed with DirEntry.Info(), which on Windows
// reuses the directory read data and on Unix lstats by path. An entry skipped by skipEntry (permission
// or not-exist) is left out; any other error fails the listing. If every name was skipped as not-exist
// (e.g. the directory was renamed mid-scan), the listing fails instead of returning an empty success.
func readDirFileInfos(dir *os.File) ([]os.FileInfo, error) {
	dirPath := dir.Name()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		// On Unix, ReadDir lstats an entry itself when the filesystem reports an unknown d_type, and
		// stops at the first failure. Only that per-entry lstat error (permission or not-exist) falls
		// back to listing by name; errors reading the directory itself are returned.
		var pe *fs.PathError
		if !errors.As(err, &pe) || pe.Op != "lstat" ||
			!(errors.Is(err, fs.ErrPermission) || errors.Is(err, fs.ErrNotExist)) {
			return nil, err
		}
		return readDirByName(dir, err)
	}
	infos := make([]os.FileInfo, 0, len(entries))
	vanished := 0
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			if !skipEntry(dirPath, e.Name(), err) {
				return nil, err
			}
			if errors.Is(err, fs.ErrNotExist) {
				vanished++
			}
			continue
		}
		infos = append(infos, fi)
	}
	if err := checkNotAllVanished(dirPath, len(entries), vanished); err != nil {
		return nil, err
	}
	return infos, nil
}

// readDirByName re-reads dir by name and lstats each entry, skipping those skipEntry allows. origErr is
// the ReadDir error returned if the directory cannot be re-read.
func readDirByName(dir *os.File, origErr error) ([]os.FileInfo, error) {
	if _, err := dir.Seek(0, io.SeekStart); err != nil {
		return nil, origErr
	}
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, origErr
	}
	dirPath := dir.Name()
	infos := make([]os.FileInfo, 0, len(names))
	vanished := 0
	for _, name := range names {
		fi, err := os.Lstat(filepath.Join(dirPath, name))
		if err != nil {
			if !skipEntry(dirPath, name, err) {
				return nil, err
			}
			if errors.Is(err, fs.ErrNotExist) {
				vanished++
			}
			continue
		}
		infos = append(infos, fi)
	}
	if err := checkNotAllVanished(dirPath, len(names), vanished); err != nil {
		return nil, err
	}
	return infos, nil
}
