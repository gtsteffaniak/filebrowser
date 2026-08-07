//go:build darwin

package analytics

import "golang.org/x/sys/unix"

func systemTotalMemoryMB() int {
	mem, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return 0
	}
	return int(mem / (1024 * 1024))
}
