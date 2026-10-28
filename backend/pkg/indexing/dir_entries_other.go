//go:build !linux

package indexing

import (
	"errors"
	"io/fs"
	"os"
)

// readDirFileInfos lists dir as FileInfos. Each entry is stat'ed with DirEntry.Info(), which on Windows
// reuses the directory read data and on Unix lstats by path. An entry skipped by skipEntry (permission
// or not-exist) is left out; any other error fails the listing. If every name was skipped as not-exist
// (e.g. the directory was renamed mid-scan), the listing fails instead of returning an empty success.
func readDirFileInfos(dir *os.File) ([]os.FileInfo, error) {
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	dirPath := dir.Name()
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