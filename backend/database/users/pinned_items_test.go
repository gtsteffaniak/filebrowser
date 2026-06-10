package users

import "testing"

func TestPinnedItemsAddRemove(t *testing.T) {
	p := make(PinnedItems)
	const source = "/data/files"
	const dir = "/photos/"
	const name = "vacation.jpg"

	p.Add(source, dir, name)
	if len(p[source][dir]) != 1 || p[source][dir][0] != name {
		t.Fatalf("add failed: %#v", p)
	}

	p.Add(source, dir, name)
	if len(p[source][dir]) != 1 {
		t.Fatalf("duplicate add should be ignored: %#v", p)
	}

	p.Remove(source, dir, name)
	if len(p) != 0 {
		t.Fatalf("remove should prune empty maps: %#v", p)
	}
}

func TestPinnedNamesForDirectory(t *testing.T) {
	u := &User{
		NonAdminEditable: NonAdminEditable{
			PinnedItems: PinnedItems{
				"/data/files": {
					"/photos/": {"vacation.jpg", "notes.txt"},
				},
			},
		},
	}

	names := u.PinnedNamesForDirectory("/data/files", "/photos/")
	if len(names) != 2 || names[0] != "vacation.jpg" || names[1] != "notes.txt" {
		t.Fatalf("unexpected names: %#v", names)
	}
	if got := u.PinnedNamesForDirectory("/missing", "/photos/"); got != nil {
		t.Fatalf("expected nil for missing source, got %#v", got)
	}
}

func TestPinnedItemsRemoveMissing(t *testing.T) {
	p := make(PinnedItems)
	p.Remove("/data/files", "/photos/", "vacation.jpg")
	if len(p) != 0 {
		t.Fatalf("remove on empty map should be no-op: %#v", p)
	}
}
