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

func getFileDetails(sys any, filePath string, useLogicalSize bool) (uint64, uint64, uint64, bool) {
	// If useLogicalSize is true, we still need inode info for hardlink detection
	// but use logical size instead of allocated size
	if sys != nil {
		if stat, ok := sys.(*syscall.Stat_t); ok {
			var realSize uint64
			if useLogicalSize {
				// Logical size mode: use actual file size
				realSize = uint64(stat.Size)
			} else {
				// Disk usage mode: use allocated size for `du`-like behavior
				realSize = uint64(stat.Blocks * 512)
			}
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

	var realSize uint64
	if useLogicalSize {
		// Logical size mode: use actual file size
		realSize = uint64(stat.Size)
	} else {
		// Disk usage mode: use allocated size for `du`-like behavior
		realSize = uint64(stat.Blocks * 512)
	}
	return realSize, uint64(stat.Nlink), stat.Ino, true
}

// handleFile processes a file and returns its size and whether it should be counted
// On Unix, always uses syscall to get size (allocated or logical based on config)
// scanner parameter is optional - if nil (API refresh), creates temporary state
func (idx *Index) handleFile(file os.FileInfo, fullCombined string, realFilePath string, isRoutineScan bool, scanner *Scanner) (size uint64, shouldCountSize bool) {
	var realSize uint64
	var nlink uint64
	var ino uint64
	canUseSyscall := false

	sys := file.Sys()
	realSize, nlink, ino, canUseSyscall = getFileDetails(sys, realFilePath, idx.Config.UseLogicalSize)

	if !canUseSyscall {
		logger.Errorf("Failed to get syscall info for file %s on Unix system - file may have been deleted or permission denied. Using file.Size() fallback.", realFilePath)
		realSize = uint64(file.Size())
	}

	if nlink > 1 {
		// It's a hard link - use scanner-specific state
		if scanner != nil {
			if _, exists := scanner.processedInodes[ino]; exists {
				// Already seen in this scan, don't count towards global total, or directory total.
				return realSize, false
			}
			// First time seeing this inode in this scan
			scanner.processedInodes[ino] = struct{}{}
			scanner.foundHardLinks[fullCombined] = realSize
		}
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
