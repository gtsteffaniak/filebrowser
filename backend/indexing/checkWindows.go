//go:build windows
// +build windows

package indexing

import (
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
