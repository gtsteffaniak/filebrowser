//go:build freebsd

package fileutils

import (
	"testing"

	"golang.org/x/sys/unix"
)

func fsidVal(a, b int32) unix.Fsid {
	return unix.Fsid{Val: [2]int32{a, b}}
}

func resolvedOK(mp string, fsid unix.Fsid) resolvedMount {
	return resolvedMount{mountpoint: mp, fsid: fsid, ok: true}
}

// TestSelectCapacityMountsOvermountedAncestorHidesNested reproduces the
// reviewed layout: /srv/child was mounted first, then /srv was overmounted.
// Getfsstat still reports the unique /srv/child record, but a lookup of that
// path lands inside the covering filesystem, so its usage must be excluded.
func TestSelectCapacityMountsOvermountedAncestorHidesNested(t *testing.T) {
	rootFs, childFs, coverFs := fsidVal(1, 1), fsidVal(2, 2), fsidVal(3, 3)
	mounts := []bsdMount{
		{fstype: "ufs", source: "/dev/da0p2", mountpoint: "/", fsid: rootFs, usage: rawUsage{total: 100, avail: 50}},
		{fstype: "ufs", source: "/dev/da1p1", mountpoint: "/srv/child", fsid: childFs, usage: rawUsage{total: 200, avail: 100}},
		{fstype: "ufs", source: "/dev/da2p1", mountpoint: "/srv", fsid: coverFs, usage: rawUsage{total: 300, avail: 150}},
	}
	lookup := func(mp string) resolvedMount {
		switch mp {
		case "/":
			return resolvedOK("/", rootFs)
		case "/srv":
			return resolvedOK("/srv", coverFs)
		case "/srv/child":
			// Hidden: resolves to the covering filesystem at /srv.
			return resolvedOK("/srv", coverFs)
		}
		return resolvedMount{}
	}

	covering, nested := selectCapacityMounts(mounts, "/srv", lookup)
	if covering == nil || covering.mountpoint != "/srv" {
		t.Fatalf("covering = %+v, want mountpoint /srv", covering)
	}
	if len(nested) != 0 {
		t.Fatalf("nested = %+v, want hidden /srv/child excluded", nested)
	}
}

// TestSelectCapacityMountsUnreachableNestedSkipped covers the same hidden
// layout when the covered path no longer exists in the new filesystem: the
// lookup fails outright and the stale record is dropped.
func TestSelectCapacityMountsUnreachableNestedSkipped(t *testing.T) {
	rootFs, coverFs := fsidVal(1, 1), fsidVal(3, 3)
	mounts := []bsdMount{
		{fstype: "ufs", mountpoint: "/", fsid: rootFs},
		{fstype: "ufs", source: "/dev/da2p1", mountpoint: "/srv", fsid: coverFs},
		{fstype: "ufs", source: "/dev/da1p1", mountpoint: "/srv/gone", fsid: fsidVal(2, 2), usage: rawUsage{total: 200, avail: 100}},
	}
	lookup := func(mp string) resolvedMount {
		switch mp {
		case "/":
			return resolvedOK("/", rootFs)
		case "/srv":
			return resolvedOK("/srv", coverFs)
		}
		return resolvedMount{} // ENOENT under the overmount
	}

	_, nested := selectCapacityMounts(mounts, "/srv", lookup)
	if len(nested) != 0 {
		t.Fatalf("nested = %+v, want unreachable /srv/gone excluded", nested)
	}
}

// TestSelectCapacityMountsStackedTopmostOnly checks that of several records
// at one mountpoint only the one a lookup resolves to is counted.
func TestSelectCapacityMountsStackedTopmostOnly(t *testing.T) {
	rootFs, lowerFs, upperFs := fsidVal(1, 1), fsidVal(4, 4), fsidVal(5, 5)
	mounts := []bsdMount{
		{fstype: "ufs", mountpoint: "/", fsid: rootFs},
		{fstype: "ufs", source: "/dev/da1p1", mountpoint: "/srv/data", fsid: lowerFs, usage: rawUsage{total: 200, avail: 100}},
		{fstype: "tmpfs", source: "tmpfs", mountpoint: "/srv/data", fsid: upperFs, usage: rawUsage{total: 60, avail: 30}},
	}
	lookup := func(mp string) resolvedMount {
		switch mp {
		case "/":
			return resolvedOK("/", rootFs)
		case "/srv/data":
			return resolvedOK("/srv/data", upperFs)
		}
		return resolvedMount{}
	}

	_, nested := selectCapacityMounts(mounts, "/srv", lookup)
	if len(nested) != 1 || nested[0].fsid != upperFs {
		t.Fatalf("nested = %+v, want only the topmost /srv/data record", nested)
	}
}

// TestSelectCapacityMountsHiddenCoveringAncestor: when the deepest ancestor
// mount of root is itself hidden by an overmount, covering must fall back to
// the deepest visible ancestor.
func TestSelectCapacityMountsHiddenCoveringAncestor(t *testing.T) {
	rootFs, overFs, staleFs := fsidVal(1, 1), fsidVal(8, 8), fsidVal(9, 9)
	mounts := []bsdMount{
		{fstype: "ufs", mountpoint: "/", fsid: rootFs},
		{fstype: "ufs", source: "/dev/da2p1", mountpoint: "/x", fsid: overFs},
		{fstype: "ufs", source: "/dev/da1p1", mountpoint: "/x/y", fsid: staleFs, usage: rawUsage{total: 200, avail: 100}},
	}
	// /x was mounted over the parent chain of /x/y: lookups of /x/y resolve
	// inside the /x filesystem.
	lookup := func(mp string) resolvedMount {
		switch mp {
		case "/":
			return resolvedOK("/", rootFs)
		case "/x":
			return resolvedOK("/x", overFs)
		case "/x/y":
			return resolvedOK("/x", overFs)
		}
		return resolvedMount{}
	}

	covering, nested := selectCapacityMounts(mounts, "/x/y/z", lookup)
	if covering == nil || covering.mountpoint != "/x" {
		t.Fatalf("covering = %+v, want deepest visible ancestor /x", covering)
	}
	if len(nested) != 0 {
		t.Fatalf("nested = %+v, want none", nested)
	}
}

// TestSelectCapacityMountsNestedVisibleKept is the baseline: a nested mount
// that still resolves to itself contributes its usage.
func TestSelectCapacityMountsNestedVisibleKept(t *testing.T) {
	rootFs, zfsFs := fsidVal(1, 1), fsidVal(7, 7)
	mounts := []bsdMount{
		{fstype: "ufs", mountpoint: "/", fsid: rootFs},
		{fstype: "zfs", source: "tank/data", mountpoint: "/srv/data", fsid: zfsFs, usage: rawUsage{total: 400, avail: 200}},
	}
	lookup := func(mp string) resolvedMount {
		switch mp {
		case "/":
			return resolvedOK("/", rootFs)
		case "/srv/data":
			return resolvedOK("/srv/data", zfsFs)
		}
		return resolvedMount{}
	}

	covering, nested := selectCapacityMounts(mounts, "/srv", lookup)
	if covering == nil || covering.mountpoint != "/" {
		t.Fatalf("covering = %+v, want /", covering)
	}
	if len(nested) != 1 || nested[0].mountpoint != "/srv/data" {
		t.Fatalf("nested = %+v, want [/srv/data]", nested)
	}
}
