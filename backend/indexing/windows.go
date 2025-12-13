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

// handleFile processes a file and returns its size and whether it should be counted
// On Windows, uses file.Size() directly (no syscall support for allocated size)
func (idx *Index) handleFile(file os.FileInfo, fullCombined string, realFilePath string) (size uint64, shouldCountSize bool) {
	// On Windows, just use the actual file size
	realSize := uint64(file.Size())
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
