//go:build !windows

package indexing

import (
	"os"
	"strings"
	"syscall"

	"github.com/gtsteffaniak/go-logger/logger"
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

func getFileDetails(sys any, filePath string) (uint64, uint64, uint64, bool) {
	// On Unix, we should always be able to get syscall info
	// First try from file.Sys() if available (fast path)
	if sys != nil {
		if stat, ok := sys.(*syscall.Stat_t); ok {
			// Use allocated size for `du`-like behavior
			realSize := uint64(stat.Blocks * 512)
			return realSize, uint64(stat.Nlink), stat.Ino, true
		}
	}

	// If file.Sys() didn't work, try direct stat syscall
	// On Unix, we should always have a file path and be able to stat
	if filePath == "" {
		// No file path available - this is an error condition on Unix
		// We cannot proceed without syscall info on Unix
		return 0, 1, 0, false
	}

	// Perform direct stat syscall - this should always work on Unix if file exists
	var stat syscall.Stat_t
	err := syscall.Stat(filePath, &stat)
	if err != nil {
		// On Unix, if stat fails, the file likely doesn't exist or we don't have permission
		// This is an error condition - we cannot proceed without syscall info on Unix
		return 0, 1, 0, false
	}

	// Use allocated size for `du`-like behavior
	realSize := uint64(stat.Blocks * 512)
	return realSize, uint64(stat.Nlink), stat.Ino, true
}

// handleFile processes a file and returns its size and whether it should be counted
// On Unix, always uses syscall to get allocated size (du-like behavior)
func (idx *Index) handleFile(file os.FileInfo, fullCombined string, realFilePath string, isRoutineScan bool) (size uint64, shouldCountSize bool) {
	var realSize uint64
	var nlink uint64
	var ino uint64
	canUseSyscall := false

	sys := file.Sys()
	realSize, nlink, ino, canUseSyscall = getFileDetails(sys, realFilePath)

	if !canUseSyscall {
		logger.Errorf("Failed to get syscall info for file %s on Unix system - file may have been deleted or permission denied. Using file.Size() fallback.", realFilePath)
		realSize = uint64(file.Size())
	}

	if nlink > 1 {
		// It's a hard link
		idx.mu.Lock()
		defer idx.mu.Unlock()
		if _, exists := idx.processedInodes[ino]; exists {
			// Already seen, don't count towards global total, or directory total.
			return realSize, false
		}
		// First time seeing this inode.
		idx.processedInodes[ino] = struct{}{}
		idx.FoundHardLinks[fullCombined] = realSize
	}
	return realSize, true // Count size for directory total.
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
