//go:build !linux

package indexing

import (
	"os"
	"path/filepath"
)

var (
	readDirListingFn = func(dir *os.File) ([]os.DirEntry, error) { return dir.ReadDir(-1) }
	dirEntryInfoFn   = func(e os.DirEntry) (os.FileInfo, error) { return e.Info() }
)

func readDirFileInfos(dir *os.File) ([]os.FileInfo, error) {
	entries, err := readDirListingFn(dir)
	if err != nil {
		return nil, err
	}
	dirPath := dir.Name()
	infos := make([]os.FileInfo, 0, len(entries))
	var skippedNotExist int
	for _, e := range entries {
		info, err := dirEntryInfoFn(e)
		if err != nil {
			if skippable(err) {
				if skipIsNotExist(err) {
					skippedNotExist++
				} else {
					warnSkippedUnreadable(filepath.Join(dirPath, e.Name()), err)
				}
				continue
			}
			return nil, err
		}
		infos = append(infos, info)
	}
	if allNamesSkippedAsNotExist(len(entries), len(infos), skippedNotExist) {
		return nil, errAllEntriesVanished
	}
	return infos, nil
}
