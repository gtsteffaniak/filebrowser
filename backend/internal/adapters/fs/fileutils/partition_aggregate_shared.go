//go:build linux || freebsd

package fileutils

import "strings"

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

// isZFSType reports whether fstype is a ZFS variant whose datasets share pool
// capacity across mounts (kernel zfs or zfsfuse).
func isZFSType(fstype string) bool {
	return fstype == "zfs" || fstype == "zfsfuse"
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
