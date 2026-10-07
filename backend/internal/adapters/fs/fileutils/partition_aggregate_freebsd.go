//go:build freebsd

package fileutils

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// bsdMount is one Getfsstat record reduced to what capacity aggregation
// needs: fstype, source, mountpoint, usage, and the mount's own fsid.
type bsdMount struct {
	fstype     string
	source     string
	mountpoint string
	usage      rawUsage
	fsid       unix.Fsid
}

// resolvedMount is what a path lookup reports for a mountpoint: the
// mountpoint of the filesystem that actually owns the path and that
// filesystem's fsid.
type resolvedMount struct {
	mountpoint string
	fsid       unix.Fsid
	ok         bool
}

// statfsMountpoint resolves which filesystem a mountpoint path lands on.
func statfsMountpoint(mp string) resolvedMount {
	var st unix.Statfs_t
	if err := unix.Statfs(mp, &st); err != nil {
		return resolvedMount{}
	}
	return resolvedMount{
		mountpoint: filepath.Clean(unix.ByteSliceToString(st.Mntonname[:])),
		fsid:       st.Fsid,
		ok:         true,
	}
}

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

	var mounts []bsdMount
	for i := range stats {
		s := &stats[i]
		avail := uint64(0)
		if s.Bavail > 0 {
			avail = uint64(s.Bavail)
		}
		mounts = append(mounts, bsdMount{
			fstype:     unix.ByteSliceToString(s.Fstypename[:]),
			source:     unix.ByteSliceToString(s.Mntfromname[:]),
			mountpoint: filepath.Clean(unix.ByteSliceToString(s.Mntonname[:])),
			usage: rawUsage{
				total: s.Blocks * s.Bsize,
				avail: avail * s.Bsize,
			},
			fsid: s.Fsid,
		})
	}

	covering, nested := selectCapacityMounts(mounts, root, statfsMountpoint)

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

// selectCapacityMounts picks the mounts contributing capacity under root: the
// filesystem covering root plus mounts nested beneath it.
//
// Getfsstat retains records that path lookups no longer reach: mounts stacked
// under a later mount at the same mountpoint, and mounts below an overmounted
// ancestor (stacked records have no documented order, so position cannot
// identify the visible one). A record counts only when a lookup on its
// mountpoint still lands on it: the resolved mountpoint must equal the
// record's and, for filesystems carrying an fsid, the resolved fsid must match
// the record's (every stacked record resolves to the topmost fsid). fsid-less
// records keep the first match at a mountpoint; stacked fsid-less mounts are
// indistinguishable.
func selectCapacityMounts(mounts []bsdMount, root string, lookup func(string) resolvedMount) (*bsdMount, []bsdMount) {
	var covering *bsdMount
	var nested []bsdMount
	claimed := make(map[string]bool)
	resolved := make(map[string]resolvedMount)
	for i := range mounts {
		m := mounts[i]
		covers := m.mountpoint == root || m.mountpoint == "/" || strings.HasPrefix(root, m.mountpoint+"/")
		nests := strings.HasPrefix(m.mountpoint, root+"/")
		if !covers && !nests {
			continue
		}
		r, seen := resolved[m.mountpoint]
		if !seen {
			r = lookup(m.mountpoint)
			resolved[m.mountpoint] = r
		}
		if !r.ok || r.mountpoint != m.mountpoint {
			continue
		}
		if m.fsid.Val[0] != 0 || m.fsid.Val[1] != 0 {
			if r.fsid != m.fsid {
				continue
			}
		} else if claimed[m.mountpoint] {
			continue
		}
		claimed[m.mountpoint] = true
		if covers && (covering == nil || len(m.mountpoint) > len(covering.mountpoint)) {
			c := m
			covering = &c
		}
		if nests {
			if isZFSType(m.fstype) && isZFSSnapshotMount(m.source, m.mountpoint) {
				continue
			}
			nested = append(nested, m)
		}
	}
	return covering, nested
}
