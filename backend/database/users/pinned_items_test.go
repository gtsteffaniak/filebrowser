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

func TestPinnedItemsMultipleItems(t *testing.T) {
	p := make(PinnedItems)
	const source = "/data/files"
	const dir = "/photos/"

	p.Add(source, dir, "a.jpg")
	p.Add(source, dir, "b.jpg")
	p.Add(source, dir, "c.jpg")

	if len(p[source][dir]) != 3 {
		t.Fatalf("expected 3 items, got %d", len(p[source][dir]))
	}

	p.Remove(source, dir, "b.jpg")
	items := p[source][dir]
	if len(items) != 2 || items[0] != "a.jpg" || items[1] != "c.jpg" {
		t.Fatalf("unexpected items after remove: %#v", items)
	}
}

func TestPinnedItemsMultipleDirectories(t *testing.T) {
	p := make(PinnedItems)
	const source = "/data/files"
	const photosDir = "/photos/"
	const docsDir = "/docs/"

	p.Add(source, photosDir, "a.jpg")
	p.Add(source, docsDir, "notes.txt")

	if len(p[source][photosDir]) != 1 || p[source][photosDir][0] != "a.jpg" {
		t.Fatalf("unexpected photos dir: %#v", p[source][photosDir])
	}
	if len(p[source][docsDir]) != 1 || p[source][docsDir][0] != "notes.txt" {
		t.Fatalf("unexpected docs dir: %#v", p[source][docsDir])
	}

	p.Remove(source, photosDir, "a.jpg")
	if _, ok := p[source][photosDir]; ok {
		t.Fatalf("photos dir should be pruned: %#v", p)
	}
	if len(p[source][docsDir]) != 1 {
		t.Fatalf("docs dir should remain: %#v", p[source][docsDir])
	}
}
