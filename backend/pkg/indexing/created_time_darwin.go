//go:build darwin

package indexing

import (
	"os"
	"syscall"
	"time"
)

func getCreatedTime(info os.FileInfo, _ string) *time.Time {
	if info == nil {
		return nil
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Birthtimespec.Sec <= 0 {
		return nil
	}
	t := time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
	return &t
}
