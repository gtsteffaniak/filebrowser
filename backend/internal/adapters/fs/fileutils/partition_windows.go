//go:build windows

package fileutils

import (
	"golang.org/x/sys/windows"
)

// GetPartitionSize returns the filesystem size for Windows systems
func GetPartitionSize(path string) (uint64, error) {
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

// GetFreeSpace returns the available free space for Windows systems
func GetFreeSpace(path string) (uint64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeBytes, totalBytes, totalFreeBytes uint64
	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytes, &totalBytes, &totalFreeBytes)
	if err != nil {
		return 0, err
	}
	return freeBytes, nil
}

// GetPartitionUsed returns the used space for Windows systems (total - free)
func GetPartitionUsed(path string) (uint64, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeBytes, totalBytes, totalFreeBytes uint64
	err = windows.GetDiskFreeSpaceEx(pathPtr, &freeBytes, &totalBytes, &totalFreeBytes)
	if err != nil {
		return 0, err
	}
	used := totalBytes - freeBytes
	return used, nil
}
