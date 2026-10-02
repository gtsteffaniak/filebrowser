//go:build linux

package indexing

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// getCreatedTime returns the file's birth time via statx, or nil if the path is empty
// or the filesystem does not report one. The link itself is queried only when info
// describes a symlink; otherwise the target is followed, matching how info was obtained.
func getCreatedTime(info os.FileInfo, realPath string) *time.Time {
	if realPath == "" {
		return nil
	}
	flags := 0
	if info != nil && info.Mode()&os.ModeSymlink != 0 {
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
