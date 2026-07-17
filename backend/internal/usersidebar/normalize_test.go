package usersidebar

import (
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func testSourceConfig(t *testing.T) {
	t.Helper()
	had := users.SourceConfigLoaded()
	t.Cleanup(func() {
		if !had {
			users.SetSourceConfig(nil)
		}
	})
	users.SetSourceConfig(&users.SourceConfigProvider{
		GetSourceByPath: func(path string) (users.SourceInfo, bool) {
			switch path {
			case "../frontend/tests/playwright-files":
				return users.SourceInfo{
					Path: "../frontend/tests/playwright-files",
					Name: "playwright + files",
				}, true
			case ".":
				return users.SourceInfo{Path: ".", Name: "docker"}, true
			case "/tests/playwright-files":
				return users.SourceInfo{Path: "/tests/playwright-files", Name: "access"}, true
			default:
				return users.SourceInfo{}, false
			}
		},
		GetSourceByName: func(name string) (users.SourceInfo, bool) {
			switch name {
			case "playwright + files":
				return users.SourceInfo{
					Path: "../frontend/tests/playwright-files",
					Name: "playwright + files",
				}, true
			case "docker":
				return users.SourceInfo{Path: ".", Name: "docker"}, true
			case "access":
				return users.SourceInfo{Path: "/tests/playwright-files", Name: "access"}, true
			default:
				return users.SourceInfo{}, false
			}
		},
	})
}

func TestNormalizeSidebarLinks_dedupesLinuxAndDockerPaths(t *testing.T) {
	testSourceConfig(t)

	in := []users.SidebarLink{
		{Name: "playwright + files", Category: "source", SourceName: "/app/frontend/tests/playwright-files", Target: "/"},
		{Name: "docker", Category: "source", SourceName: "/app/backend", Target: "/"},
		{Name: "access", Category: "source", SourceName: "/tests/playwright-files", Target: "/"},
		{Name: "playwright + files", Category: "source", SourceName: "../frontend/tests/playwright-files", Target: "/"},
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
		{Name: "access", Category: "source", SourceName: "/tests/playwright-files", Target: "/"},
	}

	out, changed := NormalizeSidebarLinks(in)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	want := []users.SidebarLink{
		{Name: "playwright + files", Category: "source", SourceName: "../frontend/tests/playwright-files", Target: "/"},
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
		{Name: "access", Category: "source", SourceName: "/tests/playwright-files", Target: "/"},
	}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("got %#v, want %#v", out, want)
	}
}

func TestNormalizeSidebarLinks_nameFallbackRemapsStalePath(t *testing.T) {
	testSourceConfig(t)

	in := []users.SidebarLink{
		{Name: "playwright + files", Category: "source", SourceName: "/app/frontend/tests/playwright-files", Target: "/"},
	}

	out, changed := NormalizeSidebarLinks(in)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].SourceName != "../frontend/tests/playwright-files" {
		t.Fatalf("SourceName = %q", out[0].SourceName)
	}
	if out[0].Name != "playwright + files" {
		t.Fatalf("Name = %q", out[0].Name)
	}
}

func TestNormalizeSidebarLinks_idempotentOnCanonicalLinks(t *testing.T) {
	testSourceConfig(t)

	in := []users.SidebarLink{
		{Name: "playwright + files", Category: "source", SourceName: "../frontend/tests/playwright-files", Target: "/"},
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
		{Name: "access", Category: "source", SourceName: "/tests/playwright-files", Target: "/"},
	}

	out, changed := NormalizeSidebarLinks(in)
	if changed {
		t.Fatal("expected changed=false for canonical links")
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("got %#v, want %#v", out, in)
	}

	out2, changed2 := NormalizeSidebarLinks(out)
	if changed2 {
		t.Fatal("expected second pass unchanged")
	}
	if !reflect.DeepEqual(out2, in) {
		t.Fatalf("second pass got %#v", out2)
	}
}

func TestNormalizeSidebarLinks_dropsUnresolvableSourceLink(t *testing.T) {
	testSourceConfig(t)

	in := []users.SidebarLink{
		{Name: "ghost", Category: "source", SourceName: "/no/such/path", Target: "/"},
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
	}

	out, changed := NormalizeSidebarLinks(in)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].Name != "docker" {
		t.Fatalf("remaining link = %#v", out[0])
	}
}

func TestNormalizeSidebarLinks_dedupesToolsLink(t *testing.T) {
	testSourceConfig(t)

	tools := users.SidebarLink{Name: "Tools", Category: "tool", Target: "/tools", Icon: "build"}
	in := []users.SidebarLink{
		tools,
		tools,
	}

	out, changed := NormalizeSidebarLinks(in)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
}

func TestNormalizeSidebarLinks_noConfigLoaded(t *testing.T) {
	had := users.SourceConfigLoaded()
	t.Cleanup(func() {
		if !had {
			users.SetSourceConfig(nil)
		}
	})
	users.SetSourceConfig(nil)

	in := []users.SidebarLink{{Name: "x", Category: "source", SourceName: "."}}
	out, changed := NormalizeSidebarLinks(in)
	if changed {
		t.Fatal("expected changed=false when source config not loaded")
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("got %#v", out)
	}
}
