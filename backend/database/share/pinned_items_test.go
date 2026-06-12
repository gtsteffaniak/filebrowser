package share

import "testing"

func TestSharePinnedItemsAddRemove(t *testing.T) {
	p := make(PinnedItems)
	const dir = "/"
	const name = "vacation.jpg"

	p.Add(dir, name)
	if len(p[dir]) != 1 || p[dir][0] != name {
		t.Fatalf("add failed: %#v", p)
	}

	p.Add(dir, name)
	if len(p[dir]) != 1 {
		t.Fatalf("duplicate add should be ignored: %#v", p)
	}

	p.Remove(dir, name)
	if len(p) != 0 {
		t.Fatalf("remove should prune empty entries: %#v", p)
	}
}

func TestSharePinnedNamesForDirectory(t *testing.T) {
	p := PinnedItems{
		"/photos/": {"vacation.jpg", "notes.txt"},
	}

	names := p.NamesForDirectory("/photos/")
	if len(names) != 2 || names[0] != "vacation.jpg" || names[1] != "notes.txt" {
		t.Fatalf("unexpected names: %#v", names)
	}
	if got := p.NamesForDirectory("/missing/"); got != nil {
		t.Fatalf("expected nil for missing directory, got %#v", got)
	}
}

func TestShareRelativeDir(t *testing.T) {
	link := &Link{CommonShare: CommonShare{Path: "/share/"}}

	got, err := link.ShareRelativeDir("/share/")
	if err != nil {
		t.Fatalf("ShareRelativeDir root: %v", err)
	}
	if got != "/" {
		t.Fatalf("got %q, want /", got)
	}

	got, err = link.ShareRelativeDir("/share/test [file2]/")
	if err != nil {
		t.Fatalf("ShareRelativeDir subdir: %v", err)
	}
	if got != "/test [file2]/" {
		t.Fatalf("got %q, want /test [file2]/", got)
	}
}
