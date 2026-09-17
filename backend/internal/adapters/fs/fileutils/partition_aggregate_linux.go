//go:build linux

package fileutils

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// GetAggregatedPartitionUsage returns total and used bytes summed across distinct
// filesystems mounted at or under root (deduped by major:minor from mountinfo).
// Falls back to a single-path probe when mountinfo cannot be read.
func GetAggregatedPartitionUsage(root string) (total, used uint64, err error) {
	if root == "" {
		return 0, 0, fmt.Errorf("empty path")
	}
	root = filepath.Clean(root)

	paths, parseErr := distinctMountPathsUnder(root)
	if parseErr != nil || len(paths) == 0 {
		return singlePathPartitionUsage(root)
	}

	var firstErr error
	for _, path := range paths {
		t, u, stErr := partitionUsageAt(path)
		if stErr != nil {
			if firstErr == nil {
				firstErr = stErr
			}
			continue
		}
		total += t
		used += u
	}

	if total == 0 && used == 0 {
		if firstErr != nil {
			return 0, 0, firstErr
		}
		return singlePathPartitionUsage(root)
	}
	return total, used, nil
}

func singlePathPartitionUsage(root string) (total, used uint64, err error) {
	total, err = GetPartitionSize(root)
	if err != nil {
		return 0, 0, err
	}
	used, err = GetPartitionUsed(root)
	if err != nil {
		return 0, 0, err
	}
	return total, used, nil
}

func distinctMountPathsUnder(root string) ([]string, error) {
	data, err := readMountinfo()
	if err != nil {
		return nil, err
	}
	paths, err := distinctMountPathsFromMountinfo(data, root)
	if err != nil {
		return nil, err
	}
	// If mountinfo had no covering mount for root, ensure root is probed.
	if len(paths) == 0 {
		return []string{root}, nil
	}
	hasRoot := false
	for _, p := range paths {
		if p == root {
			hasRoot = true
			break
		}
	}
	if !hasRoot {
		if dev, stErr := deviceIDFromPath(root); stErr == nil {
			// Avoid duplicating a device already selected under another path.
			already := false
			for _, p := range paths {
				if d, e := deviceIDFromPath(p); e == nil && d == dev {
					already = true
					break
				}
			}
			if !already {
				paths = append(paths, root)
			}
		}
	}
	return paths, nil
}

func readMountinfo() (string, error) {
	for _, path := range []string{"/proc/self/mountinfo", "/proc/1/mountinfo"} {
		b, err := os.ReadFile(path)
		if err == nil {
			return string(b), nil
		}
	}
	return "", fmt.Errorf("mountinfo not available")
}

func deviceIDFromPath(path string) (string, error) {
	var st unix.Stat_t
	if err := unix.Stat(path, &st); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", unix.Major(uint64(st.Dev)), unix.Minor(uint64(st.Dev))), nil
}

func partitionUsageAt(path string) (total, used uint64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, err
	}
	bsize := blockSize(&stat)
	total = uint64(stat.Blocks) * bsize
	free := uint64(stat.Bavail) * bsize
	if total >= free {
		used = total - free
	}
	return total, used, nil
}
