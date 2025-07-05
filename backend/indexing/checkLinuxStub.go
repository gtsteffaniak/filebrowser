//go:build !windows
// +build !windows

package indexing

import "syscall"

func CheckWindowsHidden(realpath string) bool {
	// Non-Windows platforms don't support hidden attributes in the same way
	return false
}

func getPartitionSize(path string) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	// Total size in bytes: Blocks * Block size
	total := stat.Blocks * uint64(stat.Bsize)
	return total, nil
}
