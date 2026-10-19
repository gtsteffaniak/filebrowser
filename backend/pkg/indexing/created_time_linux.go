//go:build linux

package indexing

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// getCreatedTime returns the file's birth time when available. Directory listings
// embed birth time in statxFileInfo; other paths use statx on realPath (or nil).
func getCreatedTime(info os.FileInfo, realPath string) *time.Time {
	if info == nil {
		return nil
	}
	if sxi, ok := info.(*statxFileInfo); ok {
		return sxi.birthTime()
	}
	if fxi, ok := info.(*fstatatFileInfo); ok {
		return fxi.birthTime()
	}
	if realPath == "" {
		return nil
	}
	flags := 0
	if info.Mode()&os.ModeSymlink != 0 {
		flags = unix.AT_SYMLINK_NOFOLLOW
	}
	var stx unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, realPath, flags, unix.STATX_BTIME, &stx)
	if err != nil || stx.Mask&unix.STATX_BTIME == 0 || stx.Btime.Sec <= 0 {
		return nil
	}
	t := time.Unix(stx.Btime.Sec, int64(stx.Btime.Nsec))
	return &t
}
