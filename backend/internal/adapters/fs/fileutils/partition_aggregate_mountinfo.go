//go:build linux

package fileutils

import (
	"bufio"
	"path"
	"strconv"
	"strings"
)

type mountEntry struct {
	deviceID   string
	mountpoint string
	fstype     string
	source     string
}

// mountGroup is a set of mountpoints that share one capacity pool. Groups keyed
// by filesystem identity get one probe path; shared-pool filesystems (zfs) keep
// every dataset mountpoint so per-dataset used space can be summed.
type mountGroup struct {
	key    string
	fstype string
	paths  []string
}

// capacityKey identifies mounts that share the same underlying capacity.
//   - zfs: every dataset has its own device ID but draws from the pool; group by pool.
//   - btrfs: subvolume mounts have distinct device IDs but report the whole
//     filesystem; group by mount source.
//   - everything else: device major:minor, as before.
func (e *mountEntry) capacityKey() string {
	switch e.fstype {
	case "zfs", "zfsfuse":
		return "zfs:" + zfsPoolName(e.source)
	case "btrfs":
		return "btrfs:" + e.source
	default:
		return "dev:" + e.deviceID
	}
}

// mountGroupsFromMountinfo selects the mounts contributing capacity for root:
// the filesystem that owns root, plus nested mounts under root, grouped by
// capacity key. ZFS snapshot mounts are excluded.
func mountGroupsFromMountinfo(mountinfo string, root string) ([]mountGroup, error) {
	root = path.Clean(root)
	entries, err := parseAllMounts(mountinfo)
	if err != nil {
		return nil, err
	}

	groupIndex := make(map[string]int)
	var groups []mountGroup
	addPath := func(e *mountEntry, p string) {
		key := e.capacityKey()
		i, ok := groupIndex[key]
		if !ok {
			i = len(groups)
			groupIndex[key] = i
			groups = append(groups, mountGroup{key: key, fstype: e.fstype})
		}
		// Only shared-pool filesystems (zfs) need every mountpoint; other groups
		// report identical statfs per mount, so one probe path suffices.
		if !isZFSType(groups[i].fstype) && len(groups[i].paths) > 0 {
			return
		}
		for _, existing := range groups[i].paths {
			if existing == p {
				return
			}
		}
		groups[i].paths = append(groups[i].paths, p)
	}

	var covering *mountEntry
	for i := range entries {
		e := &entries[i]
		mp := e.mountpoint
		if mp == root || strings.HasPrefix(root, mp+"/") || mp == "/" {
			// >= keeps the later entry: stacked mounts at one mountpoint are
			// shadowed by the topmost (last in mountinfo order).
			if covering == nil || len(mp) >= len(covering.mountpoint) {
				covering = e
			}
		}
	}
	if covering != nil {
		// Probe root itself for the covering filesystem: root may be a plain
		// directory inside a mount whose mountpoint is an ancestor.
		addPath(covering, root)
	} else {
		// No mount covers root (e.g. hidden bind mount); probe it directly so
		// it is not dropped when nested mounts under root do exist.
		groups = append(groups, mountGroup{key: "self:" + root, paths: []string{root}})
	}

	// Stacked mounts: only the topmost (last in mountinfo order) is reachable.
	lastAtMountpoint := make(map[string]int)
	for i := range entries {
		lastAtMountpoint[entries[i].mountpoint] = i
	}
	for i := range entries {
		e := &entries[i]
		mp := e.mountpoint
		if mp != root && !strings.HasPrefix(mp, root+"/") {
			continue
		}
		if lastAtMountpoint[mp] != i {
			continue
		}
		if isZFSType(e.fstype) && isZFSSnapshotMount(e.source, mp) {
			continue
		}
		addPath(e, mp)
	}

	return groups, nil
}

func parseAllMounts(mountinfo string) ([]mountEntry, error) {
	var out []mountEntry
	scanner := bufio.NewScanner(strings.NewReader(mountinfo))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		entry, ok := parseMountinfoLine(scanner.Text())
		if !ok {
			continue
		}
		entry.mountpoint = path.Clean(entry.mountpoint)
		out = append(out, entry)
	}
	return out, scanner.Err()
}

func parseMountinfoLine(line string) (mountEntry, bool) {
	parts := strings.SplitN(line, " - ", 2)
	if len(parts) != 2 {
		return mountEntry{}, false
	}
	fields := strings.Fields(parts[0])
	if len(fields) < 5 {
		return mountEntry{}, false
	}
	// After " - ": "fstype source super-options".
	post := strings.Fields(parts[1])
	if len(post) < 2 {
		return mountEntry{}, false
	}
	return mountEntry{
		deviceID:   fields[2],
		mountpoint: unescapeFstab(fields[4]),
		fstype:     post[0],
		source:     unescapeFstab(post[1]),
	}, true
}

func unescapeFstab(path string) string {
	var b strings.Builder
	b.Grow(len(path))
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' && i+3 < len(path) {
			if n, err := strconv.ParseUint(path[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(path[i])
	}
	return b.String()
}
