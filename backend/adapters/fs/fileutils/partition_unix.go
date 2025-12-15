//go:build !windows

package fileutils

import (
	"syscall"
)

// GetPartitionSize returns the filesystem size for Unix systems
func GetPartitionSize(path string) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	// Total size in bytes: Blocks * Block size
	total := uint64(stat.Blocks) * uint64(stat.Bsize)
	return total, nil
}

// GetFreeSpace returns the available free space for Unix systems
func GetFreeSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	// Available free space in bytes: Available blocks * Block size
	free := uint64(stat.Bavail) * uint64(stat.Bsize)
	return free, nil
}
