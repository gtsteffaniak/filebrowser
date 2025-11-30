//go:build windows

package indexing

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

func CheckWindowsHidden(realpath string) bool {
	// Convert the realpath to a UTF-16 pointer
	pointer, err := windows.UTF16PtrFromString(realpath)
	if err != nil {
		return false
	}

	// Get the file attributes
	attributes, err := windows.GetFileAttributes(pointer)
	if err != nil {
		return false
	}

	// Check if the hidden attribute is set
	if attributes&windows.FILE_ATTRIBUTE_HIDDEN != 0 {
		return true
	}

	// Optional: Check for system attribute
	if attributes&windows.FILE_ATTRIBUTE_SYSTEM != 0 {
		return true
	}
	return false
}

func getPartitionSize(path string) (uint64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeBytes, totalBytes, totalFreeBytes uint64
	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytes, &totalBytes, &totalFreeBytes)
	if err != nil {
		return 0, err
	}
	return totalBytes, nil
}

func getFileDetails(sys any, filePath string) (uint64, uint64, uint64, bool) {
	// On Windows, os.FileInfo.Sys() returns *syscall.Win32FileAttributeData,
	// which does not contain inode or link count information.
	// We return false to indicate that we should use the fallback mechanism.
	// filePath parameter is ignored on Windows
	return 0, 1, 0, false
}

// handleFile processes a file and returns its size and whether it should be counted
// On Windows, uses file.Size() directly (no syscall support for allocated size)
// isRoutineScan: if true, updates the global totalSize; if false (API calls), only returns size
func (idx *Index) handleFile(file os.FileInfo, fullCombined string, realFilePath string, isRoutineScan bool) (size uint64, shouldCountSize bool) {
	// On Windows, just use the actual file size
	realSize := uint64(file.Size())

	// Windows doesn't support hard links in the same way, so always count size
	// Only update totalSize during routine scans (not API calls)
	if isRoutineScan {
	idx.mu.Lock()
	idx.totalSize += realSize
	idx.mu.Unlock()
	}
	return realSize, true
}

// input should be non-index path.
func (idx *Index) MakeIndexPathPlatform(path string) string {
	split := strings.Split(path, "\\")
	if len(split) > 1 {
		path = strings.Join(split, "/")
	} else {
		path = "/" + strings.TrimPrefix(path, "/")
	}
	return path
}
