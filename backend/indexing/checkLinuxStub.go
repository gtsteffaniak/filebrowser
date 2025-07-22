//go:build !windows
// +build !windows

package indexing

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func CheckWindowsHidden(realpath string) bool {
	// Non-Windows platforms don't support hidden attributes in the same way
	return false
}

func getPartitionSize(path string) (uint64, error) {
	// Get the device containing the path
	var stat syscall.Stat_t
	err := syscall.Stat(path, &stat)
	if err != nil {
		return 0, err
	}

	// Get the device number
	dev := stat.Dev
	major := (dev >> 8) & 0xff
	minor := dev & 0xff

	// Read /proc/partitions to find the partition size
	file, err := os.Open("/proc/partitions")
	if err != nil {
		// Fallback to filesystem size if we can't read /proc/partitions
		return getFilesystemSize(path)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "major") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		// Parse major and minor device numbers
		majorNum, err1 := strconv.ParseUint(fields[0], 10, 32)
		minorNum, err2 := strconv.ParseUint(fields[1], 10, 32)
		blocks, err3 := strconv.ParseUint(fields[2], 10, 64)

		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}

		// Check if this matches our device
		if uint32(majorNum) == uint32(major) && uint32(minorNum) == uint32(minor) {
			// blocks are in 1024-byte units in /proc/partitions
			return blocks * 1024, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("error reading /proc/partitions: %v", err)
	}

	// Fallback to filesystem size if partition not found
	return getFilesystemSize(path)
}

// getFilesystemSize returns the filesystem size (fallback method)
func getFilesystemSize(path string) (uint64, error) {
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
