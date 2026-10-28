//go:build !linux

package indexing

import "os"

func readDirFileInfos(dir *os.File) ([]os.FileInfo, error) {
	return readDirSkippingUnreadable(dir)
}
