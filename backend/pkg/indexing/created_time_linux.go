//go:build linux

package indexing

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func getCreatedTime(_ os.FileInfo, realPath string) *time.Time {
	if realPath == "" {
		return nil
	}
	var stx unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, realPath, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BTIME, &stx)
	if err != nil || stx.Mask&unix.STATX_BTIME == 0 || stx.Btime.Sec <= 0 {
		return nil
	}
	t := time.Unix(stx.Btime.Sec, int64(stx.Btime.Nsec))
	return &t
}
