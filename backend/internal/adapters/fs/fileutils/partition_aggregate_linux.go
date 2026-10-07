//go:build linux

package fileutils

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// GetPartitionUsageVariants returns two capacity views of root:
//   - aggregate: total/used summed across distinct capacity groups mounted at or
//     under root, deduped so shared-pool filesystems (ZFS datasets, btrfs
//     subvolumes) are counted once per pool.
//   - rootOnly: a single statfs of root, ignoring nested mounts.
//
// Falls back to the single-path probe for both when mountinfo cannot be read.
func GetPartitionUsageVariants(root string) (aggregate, rootOnly PartitionUsage, err error) {
	if root == "" {
		return aggregate, rootOnly, fmt.Errorf("empty path")
	}
	root = filepath.Clean(root)

	rootOnly, rootErr := singlePathPartitionUsage(root)

	groups, parseErr := mountGroupsUnder(root)
	if parseErr != nil || len(groups) == 0 {
		if rootErr != nil {
			return aggregate, rootOnly, rootErr
		}
		return rootOnly, rootOnly, nil
	}

	var firstErr error
	for _, group := range groups {
		stats := make([]rawUsage, 0, len(group.paths))
		for _, p := range group.paths {
			stat, stErr := partitionStatfs(p)
			if stErr != nil {
				if firstErr == nil {
					firstErr = stErr
				}
				continue
			}
			stats = append(stats, stat)
		}
		u := combineGroupUsage(stats)
		aggregate.Total += u.Total
		aggregate.Used += u.Used
	}

	if aggregate.Total == 0 && aggregate.Used == 0 {
		if firstErr != nil {
			return aggregate, rootOnly, firstErr
		}
		aggregate = rootOnly
	}
	return aggregate, rootOnly, rootErr
}

func mountGroupsUnder(root string) ([]mountGroup, error) {
	data, err := readMountinfo()
	if err != nil {
		return nil, err
	}
	return mountGroupsFromMountinfo(data, root)
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

func partitionStatfs(path string) (rawUsage, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return rawUsage{}, err
	}
	bsize := blockSize(&stat)
	return rawUsage{
		total: uint64(stat.Blocks) * bsize,
		avail: uint64(stat.Bavail) * bsize,
	}, nil
}
