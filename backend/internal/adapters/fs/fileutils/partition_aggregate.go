package fileutils

import "strings"

// PartitionUsage is a capacity view of a source path: total and used bytes.
type PartitionUsage struct {
	Total uint64 `json:"total"`
	Used  uint64 `json:"used"`
}

// rawUsage is a single filesystem measurement in bytes.
type rawUsage struct {
	total uint64
	avail uint64
}

// combineGroupUsage folds statfs results for mounts that share one capacity pool
// into a single total/used pair. Per-mount used (total-avail) is summed, while
// shared free space is counted once via the largest reported avail. This keeps
// ZFS datasets or btrfs subvolumes from multiplying the pool's capacity by the
// number of mounts under a source.
func combineGroupUsage(stats []rawUsage) PartitionUsage {
	var used, avail uint64
	for _, s := range stats {
		if s.total > s.avail {
			used += s.total - s.avail
		}
		if s.avail > avail {
			avail = s.avail
		}
	}
	return PartitionUsage{Total: used + avail, Used: used}
}

// singlePathPartitionUsage statfs's a single path: the filesystem covering it.
func singlePathPartitionUsage(path string) (PartitionUsage, error) {
	total, err := GetPartitionSize(path)
	if err != nil {
		return PartitionUsage{}, err
	}
	used, err := GetPartitionUsed(path)
	if err != nil {
		return PartitionUsage{}, err
	}
	return PartitionUsage{Total: total, Used: used}, nil
}

// GetAggregatedPartitionUsage returns total and used bytes summed across distinct
// capacity groups mounted at or under root. See GetPartitionUsageVariants.
func GetAggregatedPartitionUsage(root string) (total, used uint64, err error) {
	aggregate, _, err := GetPartitionUsageVariants(root)
	if err != nil {
		return 0, 0, err
	}
	return aggregate.Total, aggregate.Used, nil
}

// zfsPoolName returns the pool component of a ZFS mount source ("pool/ds@snap" -> "pool").
func zfsPoolName(source string) string {
	if i := strings.IndexAny(source, "/@"); i >= 0 {
		return source[:i]
	}
	return source
}

// isZFSSnapshotMount reports whether a ZFS mount is a snapshot (.zfs snapdir or
// an "@snapshot" source). Snapshots share pool space with the live datasets and
// are excluded from capacity aggregation.
func isZFSSnapshotMount(source, mountpoint string) bool {
	return strings.Contains(source, "@") || strings.Contains(mountpoint, "/.zfs/")
}
