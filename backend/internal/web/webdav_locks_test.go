package web

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/net/webdav"
)

func TestScopedLocksOverlappingScopes(t *testing.T) {
	root := scopeTestRoot(t)
	now := time.Now()
	base := webdav.NewMemLS()
	broad := newScopedLockSystem(base, root)
	narrow := newScopedLockSystem(base, root+"/team")
	token, err := broad.Create(now, webdav.LockDetails{Root: "/team/file", Duration: time.Hour, ZeroDepth: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := narrow.Create(now, webdav.LockDetails{Root: "/file", Duration: time.Hour}); err != webdav.ErrLocked {
		t.Fatalf("overlap: %v", err)
	}
	release, err := narrow.Confirm(now, "/file", "", webdav.Condition{Token: token})
	if err != nil {
		t.Fatal(err)
	}
	release()
	details, err := narrow.Refresh(now, token, time.Minute)
	if err != nil || details.Root != "/file" {
		t.Fatalf("refresh: %+v %v", details, err)
	}
	if err := narrow.Unlock(now, token); err != nil {
		t.Fatal(err)
	}
}

func TestScopedLocksSeparateScopes(t *testing.T) {
	root := scopeTestRoot(t)
	now := time.Now()
	base := webdav.NewMemLS()
	a := newScopedLockSystem(base, root+"/a")
	b := newScopedLockSystem(base, root+"/b")
	token, err := a.Create(now, webdav.LockDetails{Root: "/file", Duration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Create(now, webdav.LockDetails{Root: "/file", Duration: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Refresh(now, token, time.Hour); err != webdav.ErrNoSuchLock {
		t.Fatalf("foreign refresh: %v", err)
	}
	if err := b.Unlock(now, token); err != webdav.ErrNoSuchLock {
		t.Fatalf("foreign unlock: %v", err)
	}
	if _, err := a.Refresh(now, token, time.Hour); err != nil {
		t.Fatalf("foreign calls changed lock: %v", err)
	}
}

func TestScopedLocksParentDepth(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(map[bool]string{false: "infinite", true: "zero"}[zero], func(t *testing.T) {
			root := scopeTestRoot(t)
			now := time.Now()
			base := webdav.NewMemLS()
			parent := newScopedLockSystem(base, root)
			child := newScopedLockSystem(base, root+"/team")
			token, err := parent.Create(now, webdav.LockDetails{Root: "/team", Duration: time.Hour, ZeroDepth: zero})
			if err != nil {
				t.Fatal(err)
			}
			_, err = child.Create(now, webdav.LockDetails{Root: "/file", Duration: time.Hour})
			if !zero && err != webdav.ErrLocked {
				t.Fatalf("parent did not lock child: %v", err)
			}
			if zero && err != nil {
				t.Fatalf("zero-depth blocked child: %v", err)
			}
			release, err := child.Confirm(now, "/file", "", webdav.Condition{Token: token})
			if zero {
				if err != webdav.ErrConfirmationFailed {
					t.Fatalf("zero-depth confirmed child: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				release()
			}
		})
	}
}

func TestScopedLocksRejectParentRefresh(t *testing.T) {
	root := scopeTestRoot(t)
	now := time.Now()
	base := webdav.NewMemLS()
	parent := newScopedLockSystem(base, root)
	child := newScopedLockSystem(base, root+"/team")
	token, err := parent.Create(now, webdav.LockDetails{Root: "/", Duration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := child.Refresh(now, token, time.Hour); err != webdav.ErrNoSuchLock {
		t.Fatal(err)
	}
	if err := child.Unlock(now, token); err != webdav.ErrNoSuchLock {
		t.Fatal(err)
	}
}

func TestScopedLockPathMapping(t *testing.T) {
	root := scopeTestRoot(t)
	s := newScopedLockSystem(webdav.NewMemLS(), root+"/team").(*scopedLockSystem)
	for _, name := range []string{"/../../file", "../../file", "/file"} {
		if got, err := s.physical(name); err != nil || got != filepath.ToSlash(root+"/team/file") {
			t.Fatalf("%q => %q", name, got)
		}
	}
	if _, ok := s.relative(filepath.ToSlash(root + "/team-other/file")); ok {
		t.Fatal("prefix sibling is visible")
	}
}

func scopeTestRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"team", "a", "b"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestScopedLocksUnsupportedConditions(t *testing.T) {
	now := time.Now()
	root := scopeTestRoot(t)
	ls := newScopedLockSystem(webdav.NewMemLS(), root)
	token, err := ls.Create(now, webdav.LockDetails{Root: "/file", Duration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	for _, condition := range []webdav.Condition{{Token: token, Not: true}, {Token: token, ETag: "etag"}, {ETag: "etag"}} {
		release, err := ls.Confirm(now, "/file", "", condition)
		if release != nil || err != webdav.ErrConfirmationFailed {
			t.Fatalf("unsupported condition accepted: %+v %v", condition, err)
		}
	}
	release, err := ls.Confirm(now, "/file", "", webdav.Condition{Token: token})
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestScopedLocksRootAliasAndChildSymlinks(t *testing.T) {
	now := time.Now()
	root := scopeTestRoot(t)
	base := webdav.NewMemLS()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	canonical := newScopedLockSystem(base, root+"/team", root)
	aliased := newScopedLockSystem(base, alias+"/team", alias)
	token, err := canonical.Create(now, webdav.LockDetails{Root: "/file", Duration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := aliased.Create(now, webdav.LockDetails{Root: "/file", Duration: time.Hour}); err != webdav.ErrLocked {
		t.Fatalf("root alias bypassed lock: %v", err)
	}
	if err := aliased.Unlock(now, token); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root+"/team", root+"/link"); err != nil {
		t.Fatal(err)
	}
	broad := newScopedLockSystem(base, root, root)
	if _, err := broad.Create(now, webdav.LockDetails{Root: "/link/file", Duration: time.Hour}); err != webdav.ErrLocked {
		t.Fatalf("child symlink allowed: %v", err)
	}
	narrowed := newScopedLockSystem(base, root+"/link", root)
	if _, err := narrowed.Create(now, webdav.LockDetails{Root: "/file", Duration: time.Hour}); err != webdav.ErrLocked {
		t.Fatalf("scope symlink allowed: %v", err)
	}
	if _, err := broad.Create(now, webdav.LockDetails{Root: "/new/child/file", Duration: time.Hour}); err != nil {
		t.Fatalf("missing parents rejected: %v", err)
	}
}

func TestScopedLocksRecursiveMutation(t *testing.T) {
	for _, recursive := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "recursive"}[recursive], func(t *testing.T) {
			now := time.Now()
			root := scopeTestRoot(t)
			base := webdav.NewMemLS()
			ls := newScopedLockSystem(base, root, root).(*scopedLockSystem)
			parent, err := ls.Create(now, webdav.LockDetails{Root: "/team", Duration: time.Hour, ZeroDepth: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ls.Create(now, webdav.LockDetails{Root: "/team/file", Duration: time.Hour, ZeroDepth: true}); err != nil {
				t.Fatal(err)
			}
			ls.recursiveMutation = recursive
			release, err := ls.Confirm(now, "/team", "", webdav.Condition{Token: parent})
			if recursive {
				if err != webdav.ErrConfirmationFailed {
					t.Fatalf("recursive confirmation bypassed child: %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				release()
			}
		})
	}
	now := time.Now()
	root := scopeTestRoot(t)
	base := webdav.NewMemLS()
	ls := newScopedLockSystem(base, root, root).(*scopedLockSystem)
	if _, err := ls.Create(now, webdav.LockDetails{Root: "/team/file", Duration: time.Hour, ZeroDepth: true}); err != nil {
		t.Fatal(err)
	}
	ls.recursiveMutation = true
	if _, err := ls.Create(now, webdav.LockDetails{Root: "/team", Duration: -1, ZeroDepth: true}); err != webdav.ErrLocked {
		t.Fatalf("temporary parent lock bypassed child: %v", err)
	}
}

func TestScopedLocksHeldExpiredDescendant(t *testing.T) {
	now := time.Now()
	root := scopeTestRoot(t)
	base := webdav.NewMemLS()
	ls := newScopedLockSystem(base, root, root).(*scopedLockSystem)
	parent, err := ls.Create(now, webdav.LockDetails{Root: "/team", Duration: time.Hour, ZeroDepth: true})
	if err != nil {
		t.Fatal(err)
	}
	child, err := ls.Create(now, webdav.LockDetails{Root: "/team/file", Duration: time.Second, ZeroDepth: true})
	if err != nil {
		t.Fatal(err)
	}
	release, err := ls.Confirm(now, "/team/file", "", webdav.Condition{Token: child})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	later := now.Add(2 * time.Second)
	if _, err := ls.Create(later, webdav.LockDetails{Root: "/other", Duration: time.Hour}); err != nil {
		t.Fatal(err)
	}
	ls.recursiveMutation = true
	if release, err := ls.Confirm(later, "/team", "", webdav.Condition{Token: parent}); err != webdav.ErrConfirmationFailed {
		if release != nil {
			release()
		}
		t.Fatalf("held expired child bypassed: %v", err)
	}
}

func TestScopedLocksBindTaggedIfToRequest(t *testing.T) {
	now := time.Now()
	root := scopeTestRoot(t)
	ls := newScopedLockSystem(webdav.NewMemLS(), root, root).(*scopedLockSystem)
	token, err := ls.Create(now, webdav.LockDetails{Root: "/other", Duration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	actual, err := ls.Create(now, webdav.LockDetails{Root: "/actual", Duration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	ls.enforceRequest = true
	ls.requestSource = "/actual"
	for _, method := range []string{"PUT", "DELETE", "MKCOL", "PROPPATCH", "MOVE", "COPY"} {
		ls.requestMethod = method
		second := ""
		if method == "MOVE" || method == "COPY" {
			ls.requestDestination = "/destination"
			second = "/destination"
		}
		release, err := ls.Confirm(now, "/other", second, webdav.Condition{Token: token})
		if err != webdav.ErrConfirmationFailed || release != nil {
			t.Fatalf("%s accepted unrelated tagged source: %v", method, err)
		}
	}
	ls.requestMethod = "PUT"
	release, err := ls.Confirm(now, "/actual", "", webdav.Condition{Token: actual})
	if err != nil {
		t.Fatal(err)
	}
	release()
	ls.requestMethod = "MOVE"
	if release, err := ls.Confirm(now, "/actual", "/wrong", webdav.Condition{Token: actual}); err != webdav.ErrConfirmationFailed {
		if release != nil {
			release()
		}
		t.Fatalf("wrong destination accepted: %v", err)
	}
}

func TestScopedLocksBindTokenOperationsToRequest(t *testing.T) {
	now := time.Now()
	root := scopeTestRoot(t)
	ls := newScopedLockSystem(webdav.NewMemLS(), root, root).(*scopedLockSystem)
	token, err := ls.Create(now, webdav.LockDetails{Root: "/actual", Duration: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	ls.enforceRequest = true
	ls.requestSource = "/other"
	ls.requestMethod = "LOCK"
	if _, err := ls.Refresh(now, token, time.Hour); err != webdav.ErrNoSuchLock {
		t.Fatalf("wrong URI refreshed: %v", err)
	}
	ls.requestMethod = "UNLOCK"
	if err := ls.Unlock(now, token); err != webdav.ErrNoSuchLock {
		t.Fatalf("wrong URI unlocked: %v", err)
	}
	ls.requestSource = "/actual"
	ls.requestMethod = "LOCK"
	if _, err := ls.Refresh(now, token, time.Hour); err != nil {
		t.Fatal(err)
	}
	ls.requestMethod = "UNLOCK"
	if err := ls.Unlock(now, token); err != nil {
		t.Fatal(err)
	}
}

func TestScopedLockRequestBindings(t *testing.T) {
	s := &scopedLockSystem{requestSource: "/source", requestDestination: "/destination"}
	for _, tc := range []struct {
		method, name0, name1 string
		valid                bool
	}{
		{"COPY", "", "/destination", true}, {"COPY", "/destination", "/destination", true},
		{"COPY", "/source", "/destination", false}, {"COPY", "", "/other", false},
		{"MOVE", "/source", "/destination", true}, {"MOVE", "/source", "", false},
		{"PUT", "/source", "", true}, {"PUT", "/source", "/destination", false},
	} {
		s.requestMethod = tc.method
		if got := s.matchesRequest(tc.name0, tc.name1); got != tc.valid {
			t.Fatalf("%+v: got %v", tc, got)
		}
	}
}
