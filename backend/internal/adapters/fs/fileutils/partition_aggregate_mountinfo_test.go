//go:build linux

package fileutils

import (
	"sort"
	"testing"
)

func TestDistinctMountPathsNestedDisk(t *testing.T) {
	// /srv lives on rootfs 8:1; /srv/data is a separate 2TB disk 8:16.
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"45 22 8:16 /data /srv/data rw - btrfs /dev/sdb1 rw\n" +
		"50 22 0:47 / /proc rw - proc proc rw\n"

	paths, err := distinctMountPathsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
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

	paths, err := distinctMountPathsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
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
		"45 22 8:16 / /srv/data rw - btrfs /dev/sdb1 rw\n" +
		"46 22 8:16 / /srv/data/bind rw,bind - btrfs /dev/sdb1 rw\n"

	paths, err := distinctMountPathsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 distinct devices, got %v", paths)
	}
}

func TestDistinctMountPathsSourceIsOwnMount(t *testing.T) {
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"40 22 8:2 / /srv rw - ext4 /dev/sda2 rw\n" +
		"45 40 8:16 / /srv/data rw - btrfs /dev/sdb1 rw\n"

	paths, err := distinctMountPathsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	want := []string{"/srv", "/srv/data"}
	if len(paths) != 2 || paths[0] != want[0] || paths[1] != want[1] {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestDistinctMountPathsIgnoresUnrelated(t *testing.T) {
	mountinfo := "" +
		"22 1 8:1 / / rw - ext4 /dev/sda1 rw\n" +
		"30 22 8:3 / /home rw - ext4 /dev/sda3 rw\n"

	paths, err := distinctMountPathsFromMountinfo(mountinfo, "/srv")
	if err != nil {
		t.Fatal(err)
	}
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
}
