package usersidebar

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func initDefaultsTestConfig(t *testing.T) {
	t.Helper()
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../../_docker/src/noauth/backend/config.yaml")
	settings.Env.IsPlaywright = true
}

func TestMergeDefaultLinks_appendsEnabledMissing(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := SidebarLinkDefaultsDocument{
		Items: []SidebarLinkDefaultItem{
			{
				Enabled: true,
				Link: users.SidebarLink{
					Name:       "Wiki",
					Category:   "custom",
					Target:     "/wiki",
					Icon:       "link",
				},
			},
		},
	}
	links, changed := MergeDefaultLinks(nil, nil, doc)
	if !changed || len(links) != 1 || links[0].Target != "/wiki" {
		t.Fatalf("got %#v changed=%v", links, changed)
	}
}

func TestMergeDefaultLinks_skipsInaccessibleSource(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := SidebarLinkDefaultsDocument{
		Items: []SidebarLinkDefaultItem{
			{
				Enabled: true,
				Link: users.SidebarLink{
					Name:       "exclude",
					Category:   "source",
					Target:     "/",
					SourceName: "exclude",
				},
			},
		},
	}
	links, changed := MergeDefaultLinks(nil, []users.BackendScope{{Path: "include", Scope: "/"}}, doc)
	if changed || len(links) != 0 {
		t.Fatalf("got %#v changed=%v", links, changed)
	}
}

func TestMergeEnforcedLinks_updatesNonAdmin(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := SidebarLinkDefaultsDocument{
		Items: []SidebarLinkDefaultItem{
			{
				Enforced: true,
				Link: users.SidebarLink{
					Name:     "Wiki",
					Category: "custom",
					Target:   "/wiki",
					Icon:     "link",
				},
			},
		},
	}
	links, changed := MergeEnforcedLinks(nil, nil, doc, false)
	if !changed || len(links) != 1 {
		t.Fatalf("got %#v changed=%v", links, changed)
	}
	links, changed = MergeEnforcedLinks(links, nil, doc, true)
	if changed {
		t.Fatalf("admin should be unchanged, got %#v", links)
	}
}

func TestValidateEnforcedSidebarLinks_blocksRemoval(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := SidebarLinkDefaultsDocument{
		Items: []SidebarLinkDefaultItem{
			{
				Enforced: true,
				Link: users.SidebarLink{
					Name:     "Wiki",
					Category: "custom",
					Target:   "/wiki",
				},
			},
		},
	}
	if err := ValidateEnforcedSidebarLinks(nil, nil, doc, false); err == nil {
		t.Fatal("expected error")
	}
	if err := ValidateEnforcedSidebarLinks(nil, nil, doc, true); err != nil {
		t.Fatalf("admin should pass: %v", err)
	}
}

func TestEnforcedLinkKeys_nonAdminOnly(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := SidebarLinkDefaultsDocument{
		Items: []SidebarLinkDefaultItem{
			{
				Enforced: true,
				Link: users.SidebarLink{
					Name:     "Wiki",
					Category: "custom",
					Target:   "/wiki",
				},
			},
		},
	}
	if keys := EnforcedLinkKeys(nil, doc, true); len(keys) != 0 {
		t.Fatalf("admin keys=%v", keys)
	}
	if keys := EnforcedLinkKeys(nil, doc, false); len(keys) != 1 {
		t.Fatalf("user keys=%v", keys)
	}
}
