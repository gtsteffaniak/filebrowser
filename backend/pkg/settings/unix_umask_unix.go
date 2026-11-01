//go:build unix

package settings

import "syscall"

func unixUmask(mask int) int {
	return syscall.Umask(mask)
}
