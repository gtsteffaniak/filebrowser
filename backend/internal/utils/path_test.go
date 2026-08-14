package utils

import (
	"path/filepath"
	"testing"
)

func TestJoinScopedIndexPath_absoluteRelDoesNotDropScope(t *testing.T) {
	const scope = "/home/alice"
	got := JoinScopedIndexPath(scope, "/etc/passwd")
	want := "/home/alice/etc/passwd"
	if got != want {
		t.Fatalf("JoinScopedIndexPath(%q, /etc/passwd) = %q, want %q", scope, got, want)
	}
}

func TestJoinScopedIndexPath_traversalResolvedUnderScope(t *testing.T) {
	const scope = "/home/alice"
	got := JoinScopedIndexPath(scope, "/../../../etc/passwd")
	want := "/home/alice/etc/passwd"
	if got != want {
		t.Fatalf("JoinScopedIndexPath(%q, /../../../etc/passwd) = %q, want %q", scope, got, want)
	}
}

func TestJoinScopedIndexPath_relativeTraversalWithoutLeadingSlash(t *testing.T) {
	const scope = "/home/alice"
	got := JoinScopedIndexPath(scope, "../../../etc/passwd")
	want := "/home/alice/etc/passwd"
	if got != want {
		t.Fatalf("JoinScopedIndexPath(%q, ../../../etc/passwd) = %q, want %q", scope, got, want)
	}
}

func TestJoinScopedIndexPath_rootScope(t *testing.T) {
	got := JoinScopedIndexPath("/", "projects/acme/file.txt")
	if got != "/projects/acme/file.txt" {
		t.Fatalf("got %q want /projects/acme/file.txt", got)
	}
}

func TestJoinUnderSourceRoot_absoluteIndexPathStaysUnderSource(t *testing.T) {
	const sourceRoot = "/srv/mount"
	got := JoinUnderSourceRoot(sourceRoot, "/etc/passwd")
	want := filepath.Join(sourceRoot, "etc/passwd")
	if got != want {
		t.Fatalf("JoinUnderSourceRoot = %q, want %q", got, want)
	}
}

func TestJoinUnderSourceRoot_relativeIndexPath(t *testing.T) {
	const sourceRoot = "/srv/mount"
	got := JoinUnderSourceRoot(sourceRoot, "/home/alice/projects/foo")
	want := filepath.Join(sourceRoot, "home/alice/projects/foo")
	if got != want {
		t.Fatalf("JoinUnderSourceRoot = %q, want %q", got, want)
	}
}

func TestJoinUnderSourceRoot_relativeTraversalWithoutLeadingSlash(t *testing.T) {
	const sourceRoot = "/srv/mount"
	got := JoinUnderSourceRoot(sourceRoot, "../../../etc/passwd")
	want := filepath.Join(sourceRoot, "etc/passwd")
	if got != want {
		t.Fatalf("JoinUnderSourceRoot = %q, want %q", got, want)
	}
}
