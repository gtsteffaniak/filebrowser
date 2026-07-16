//go:build windows

package analytics

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

func systemTotalMemoryMB() int {
	var status windows.MemoryStatusEx
	status.Length = uint32(unsafe.Sizeof(status))
	if err := windows.GlobalMemoryStatusEx(&status); err != nil {
		return 0
	}
	return int(status.TotalPhys / (1024 * 1024))
}
