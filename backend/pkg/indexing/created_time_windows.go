//go:build windows

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
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return nil
	}
	nsec := stat.CreationTime.Nanoseconds()
	if nsec <= 0 {
		return nil
	}
	t := time.Unix(0, nsec)
	return &t
}
