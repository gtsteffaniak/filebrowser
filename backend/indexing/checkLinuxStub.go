//go:build !windows
// +build !windows

package indexing

import (
	"syscall"
)

func CheckWindowsHidden(realpath string) bool {
	// Non-Windows platforms don't support hidden attributes in the same way
	return false
}

// getFilesystemSize returns the filesystem size (fallback method)
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

func getFileDetails(sys any) (uint64, uint64, uint64, bool) {
	if stat, ok := sys.(*syscall.Stat_t); ok {
		// Use allocated size for `du`-like behavior
		realSize := uint64(stat.Blocks * 512)
		return realSize, uint64(stat.Nlink), stat.Ino, true
	}
	return 0, 1, 0, false
}
