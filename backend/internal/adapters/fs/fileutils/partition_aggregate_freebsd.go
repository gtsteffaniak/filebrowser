//go:build freebsd

package fileutils

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// GetPartitionUsageVariants returns the aggregate and root-only capacity views
// of root. FreeBSD has no mountinfo; Getfsstat already carries per-mount statfs
// data, so grouping happens on the result set directly (ZFS datasets grouped by
// pool name, everything else by fsid).
func GetPartitionUsageVariants(root string) (aggregate, rootOnly PartitionUsage, err error) {
	if root == "" {
		return aggregate, rootOnly, fmt.Errorf("empty path")
	}
	root = filepath.Clean(root)

	rootOnly, rootErr := singlePathPartitionUsage(root)

	count, statErr := unix.Getfsstat(nil, unix.MNT_WAIT)
	if statErr != nil || count <= 0 {
		if rootErr != nil {
			return aggregate, rootOnly, rootErr
		}
		return rootOnly, rootOnly, nil
	}
	stats := make([]unix.Statfs_t, count)
	if _, statErr = unix.Getfsstat(stats, unix.MNT_WAIT); statErr != nil {
		if rootErr != nil {
			return aggregate, rootOnly, rootErr
		}
		return rootOnly, rootOnly, nil
	}

	type bsdMount struct {
		fstype     string
		source     string
		mountpoint string
		usage      rawUsage
		fsid       unix.Fsid
	}
	var mounts []bsdMount
	mountpointCount := make(map[string]int)
	for i := range stats {
		s := &stats[i]
		avail := uint64(0)
		if s.Bavail > 0 {
			avail = uint64(s.Bavail)
		}
		m := bsdMount{
			fstype:     unix.ByteSliceToString(s.Fstypename[:]),
			source:     unix.ByteSliceToString(s.Mntfromname[:]),
			mountpoint: filepath.Clean(unix.ByteSliceToString(s.Mntonname[:])),
			usage: rawUsage{
				total: s.Blocks * s.Bsize,
				avail: avail * s.Bsize,
			},
			fsid: s.Fsid,
		}
		mounts = append(mounts, m)
		mountpointCount[m.mountpoint]++
	}

	// Getfsstat may return several records for one mountpoint (stacked mounts)
	// with no documented order, so position cannot identify the visible mount.
	// The visible record is the one a path lookup resolves to: statfs the
	// mountpoint and keep the record whose fsid matches.
	visibleFsid := make(map[string]unix.Fsid)
	for mp, n := range mountpointCount {
		if n < 2 {
			continue
		}
		var st unix.Statfs_t
		if err := unix.Statfs(mp, &st); err == nil {
			visibleFsid[mp] = st.Fsid
		}
	}

	var covering *bsdMount
	var nested []bsdMount
	claimed := make(map[string]bool)
	for i := range mounts {
		m := mounts[i]
		if mountpointCount[m.mountpoint] > 1 {
			fsid, ok := visibleFsid[m.mountpoint]
			if !ok || m.fsid != fsid || claimed[m.mountpoint] {
				continue
			}
			claimed[m.mountpoint] = true
		}
		if m.mountpoint == root || strings.HasPrefix(root, m.mountpoint+"/") || m.mountpoint == "/" {
			if covering == nil || len(m.mountpoint) > len(covering.mountpoint) {
				c := m
				covering = &c
			}
		}
		if strings.HasPrefix(m.mountpoint, root+"/") {
			if isZFSType(m.fstype) && isZFSSnapshotMount(m.source, m.mountpoint) {
				continue
			}
			nested = append(nested, m)
		}
	}

	key := func(m bsdMount) string {
		if isZFSType(m.fstype) {
			return "zfs:" + zfsPoolName(m.source)
		}
		if m.fsid.Val[0] == 0 && m.fsid.Val[1] == 0 {
			// fsid-less filesystems (e.g. some tmpfs mounts): keep per mountpoint.
			return "mp:" + m.mountpoint
		}
		return fmt.Sprintf("fsid:%x:%x", m.fsid.Val[0], m.fsid.Val[1])
	}

	groups := make(map[string][]rawUsage)
	fstypes := make(map[string]string)
	add := func(m bsdMount) {
		k := key(m)
		fstypes[k] = m.fstype
		groups[k] = append(groups[k], m.usage)
	}
	if covering != nil {
		add(*covering)
	}
	for _, m := range nested {
		add(m)
	}
	if len(groups) == 0 {
		if rootErr != nil {
			return aggregate, rootOnly, rootErr
		}
		return rootOnly, rootOnly, nil
	}

	for k, stats := range groups {
		u := combineGroupUsage(stats)
		if !isZFSType(fstypes[k]) && len(stats) > 1 {
			// Non-shared-pool mounts dedupe to a single measurement.
			u = combineGroupUsage(stats[:1])
		}
		aggregate.Total += u.Total
		aggregate.Used += u.Used
	}
	return aggregate, rootOnly, rootErr
}
