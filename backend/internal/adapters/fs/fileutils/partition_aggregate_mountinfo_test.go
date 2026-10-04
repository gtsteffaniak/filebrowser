//go:build linux

package fileutils

import (
	"sort"
	"testing"
)

// allGroupPaths flattens groups into a sorted list of probe paths.
func allGroupPaths(groups []mountGroup) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g.paths...)
	}
	sort.Strings(out)
	return out
}

func findGroup(groups []mountGroup, prefix string) *mountGroup {
	for i := range groups {
		if len(groups[i].key) >= len(prefix) && groups[i].key[:len(prefix)] == prefix {
			return &groups[i]
		}
	}
	return nil
}

func TestDistinctMountPathsNestedDisk(t *testing.T) {
	// /srv lives on rootfs 8:1; /srv/data is a separate 2TB disk 8:16.
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"45 22 8:16 /data /srv/data rw - btrfs /dev/sdb1 rw\n" +
		"50 22 0:47 / /proc rw - proc proc rw\n"

	groups, err := mountGroupsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	paths := allGroupPaths(groups)
	want := []string{"/srv", "/srv/data"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %v, want %v", paths, want)
		}
	}
}

func TestDistinctMountPathsOvermountSamePathDeduped(t *testing.T) {
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"40 22 8:2 / /srv rw - ext4 /dev/sda2 rw\n" +
		"45 40 8:16 / /srv/data rw - btrfs /dev/sdb1 rw\n" +
		"46 45 0:47 / /srv/data rw - tmpfs tmpfs rw\n"

	groups, err := mountGroupsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	paths := allGroupPaths(groups)
	want := []string{"/srv", "/srv/data"}
	if len(paths) != len(want) {
		t.Fatalf("paths = %v, want %v (no duplicate probe paths)", paths, want)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("paths = %v, want %v", paths, want)
		}
	}
}

func TestDistinctMountPathsBindSameDeviceNotDuplicated(t *testing.T) {
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"45 22 8:16 / /srv/data rw - ext4 /dev/sdb1 rw\n" +
		"46 22 8:16 / /srv/data/bind rw,bind - ext4 /dev/sdb1 rw\n"

	groups, err := mountGroupsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 distinct capacity groups, got %v", groups)
	}
}

func TestDistinctMountPathsSourceIsOwnMount(t *testing.T) {
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"40 22 8:2 / /srv rw - ext4 /dev/sda2 rw\n" +
		"45 40 8:16 / /srv/data rw - btrfs /dev/sdb1 rw\n"

	groups, err := mountGroupsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	paths := allGroupPaths(groups)
	want := []string{"/srv", "/srv/data"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestDistinctMountPathsIgnoresUnrelated(t *testing.T) {
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"30 22 8:3 / /home rw - ext4 /dev/sda3 rw\n"

	groups, err := mountGroupsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	paths := allGroupPaths(groups)
	if len(paths) != 1 || paths[0] != "/srv" {
		t.Fatalf("paths = %v, want [/srv]", paths)
	}
}

func TestParseMountinfoEscapedSpace(t *testing.T) {
	entry, ok := parseMountinfoLine(`36 35 98:0 /mnt1 /mnt\040two rw - ext3 /dev/root rw`)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if entry.mountpoint != "/mnt two" {
		t.Fatalf("mountpoint = %q, want /mnt two", entry.mountpoint)
	}
	if entry.deviceID != "98:0" {
		t.Fatalf("deviceID = %q, want 98:0", entry.deviceID)
	}
	if entry.fstype != "ext3" {
		t.Fatalf("fstype = %q, want ext3", entry.fstype)
	}
	if entry.source != "/dev/root" {
		t.Fatalf("source = %q, want /dev/root", entry.source)
	}
}

// TestMountGroupsZFSPoolDatasetsGrouped reproduces issue #3025: every ZFS
// dataset gets its own device ID, so major:minor dedup counts pool capacity
// once per dataset. They must share one capacity group.
func TestMountGroupsZFSPoolDatasetsGrouped(t *testing.T) {
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"40 22 0:55 / /srv/pool rw - zfs pool rw\n" +
		"41 40 0:56 / /srv/pool/ds01 rw - zfs pool/ds01 rw\n" +
		"42 41 0:57 / /srv/pool/ds01/a rw - zfs pool/ds01/a rw\n" +
		"43 40 0:58 / /srv/pool/ds02 rw - zfs pool/ds02 rw\n" +
		"44 22 0:59 / /other/pool2 rw - zfs pool2 rw\n"

	groups, err := mountGroupsFromMountinfo(mountinfo, "/srv/pool")
	if err != nil {
		t.Fatal(err)
	}
	g := findGroup(groups, "zfs:pool")
	if g == nil || g.key != "zfs:pool" {
		t.Fatalf("expected one zfs:pool group, got %v", groups)
	}
	want := []string{"/srv/pool", "/srv/pool/ds01", "/srv/pool/ds01/a", "/srv/pool/ds02"}
	got := append([]string{}, g.paths...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("zfs group paths = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("zfs group paths = %v, want %v", got, want)
		}
	}
	// The unrelated second pool must not join the group.
	if g2 := findGroup(groups, "zfs:pool2"); g2 != nil {
		t.Fatalf("unrelated pool2 should not be included: %v", groups)
	}
}

func TestMountGroupsZFSSnapshotsSkipped(t *testing.T) {
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"40 22 0:55 / /srv/pool rw - zfs pool rw\n" +
		"41 40 0:56 / /srv/pool/ds01 rw - zfs pool/ds01 rw\n" +
		"42 41 0:57 / /srv/pool/ds01/.zfs/snapshot/snap1 rw - zfs pool/ds01@snap1 rw\n"

	groups, err := mountGroupsFromMountinfo(mountinfo, "/srv/pool")
	if err != nil {
		t.Fatal(err)
	}
	paths := allGroupPaths(groups)
	for _, p := range paths {
		if p == "/srv/pool/ds01/.zfs/snapshot/snap1" {
			t.Fatalf("snapshot mount should be skipped: %v", paths)
		}
	}
	if len(paths) != 2 {
		t.Fatalf("paths = %v, want 2 live datasets", paths)
	}
}

func TestMountGroupsBtrfsSubvolumesDeduped(t *testing.T) {
	// Each btrfs subvolume mount has a distinct anonymous device but reports
	// whole-filesystem statfs; they must collapse into one group.
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"40 22 0:33 /@ /srv rw - btrfs /dev/sdb1 rw\n" +
		"41 40 0:34 /@home /srv/home rw - btrfs /dev/sdb1 rw\n" +
		"42 40 0:35 /@data /srv/data rw - btrfs /dev/sdb1 rw\n"

	groups, err := mountGroupsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	g := findGroup(groups, "btrfs:")
	if g == nil || g.key != "btrfs:/dev/sdb1" {
		t.Fatalf("expected one btrfs group, got %v", groups)
	}
	// Subvolume mounts report identical fs-wide statfs; one probe path suffices.
	if len(g.paths) != 1 || g.paths[0] != "/srv" {
		t.Fatalf("btrfs group paths = %v, want [/srv]", g.paths)
	}
	if len(groups) != 1 {
		t.Fatalf("expected exactly 1 group for /srv, got %v", groups)
	}
}

// TestCombineGroupUsageZFSPool verifies the arithmetic from #3025: 26 datasets
// each reporting ~19.7T avail must yield used+19.7T total, not 26*19.7T+used.
func TestCombineGroupUsageZFSPool(t *testing.T) {
	const avail = uint64(19_700_000_000_000)                       // ~19.7 TiB pool free
	stats := []rawUsage{{total: 20_000_000_000_000, avail: avail}} // root ds: 0.3T used
	for i := 0; i < 25; i++ {
		stats = append(stats, rawUsage{total: avail + 100_000_000_000, avail: avail})
	}
	// One quota'd child: reports a smaller avail; must not shrink the pool.
	stats = append(stats, rawUsage{total: 1_000_000_000_000, avail: 500_000_000_000})

	u := combineGroupUsage(stats)
	wantUsed := uint64(300_000_000_000) + 25*uint64(100_000_000_000) + 500_000_000_000
	if u.Used != wantUsed {
		t.Fatalf("used = %d, want %d", u.Used, wantUsed)
	}
	if u.Total != wantUsed+avail {
		t.Fatalf("total = %d, want %d (used+single pool avail)", u.Total, wantUsed+avail)
	}
}

func TestCombineGroupUsageSingleMountIdentity(t *testing.T) {
	u := combineGroupUsage([]rawUsage{{total: 1000, avail: 400}})
	if u.Total != 1000 || u.Used != 600 {
		t.Fatalf("single mount = %+v, want total 1000 used 600", u)
	}
}
