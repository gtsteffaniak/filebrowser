//go:build !windows

package indexing

import (
	"strings"
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
		// Use actual file size, not allocated blocks
		// This matches what ls -l shows and what FileInfoFaster returns
		realSize := uint64(stat.Size)
		return realSize, uint64(stat.Nlink), stat.Ino, true
	}
	return 0, 1, 0, false
}

// platform specific rules
func (idx *Index) MakeIndexPathPlatform(path string) string {
	if idx.mock {
		// also do windows check for testing
		split := strings.Split(path, "\\")
		if len(split) > 1 {
			path = strings.Join(split, "/")
		} else {
			path = "/" + strings.TrimPrefix(path, "/")
		}
	} else {
		path = "/" + strings.TrimPrefix(path, "/")
	}
	return path
}
