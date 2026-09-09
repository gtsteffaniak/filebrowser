//go:build linux

package fileutils

import (
	"syscall"
)

func blockSize(stat *syscall.Statfs_t) uint64 {
	if stat.Frsize > 0 {
		return uint64(stat.Frsize)
	}
	return uint64(stat.Bsize)
}

// GetPartitionSize returns the filesystem size for Linux systems
func GetPartitionSize(path string) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	return uint64(stat.Blocks) * blockSize(&stat), nil
}

// GetFreeSpace returns the available free space for Linux systems
func GetFreeSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	return uint64(stat.Bavail) * blockSize(&stat), nil
}

// GetPartitionUsed returns the used space for Linux systems (total - free)
func GetPartitionUsed(path string) (uint64, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return 0, err
	}
	bsize := blockSize(&stat)
	total := uint64(stat.Blocks) * bsize
	free := uint64(stat.Bavail) * bsize
	return total - free, nil
}
