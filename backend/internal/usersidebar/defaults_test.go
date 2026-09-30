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

func TestFilterDefaultsDocumentForUser_filtersInaccessibleSources(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := SidebarLinkDefaultsDocument{
		Items: []SidebarLinkDefaultItem{
			{
				Enforced: true,
				Link: users.SidebarLink{
					Name:       "exclude",
					Category:   "source",
					Target:     "/",
					SourceName: "exclude",
				},
			},
			{
				Enforced: true,
				Link: users.SidebarLink{
					Name:       "include",
					Category:   "source",
					Target:     "/",
					SourceName: "include",
				},
			},
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
	excludeSource, ok := users.ResolveSourceKey("exclude")
	if !ok {
		t.Fatal("exclude source not in test config")
	}
	scopes := []users.BackendScope{{Path: excludeSource.Path}}
	filtered := FilterDefaultsDocumentForUser(doc, scopes)
	if len(filtered.Items) != 2 {
		t.Fatalf("len(items)=%d want 2 (exclude source + custom): %#v", len(filtered.Items), filtered.Items)
	}
}

func TestInitialSidebarLinkDefaultsDocument_enablesAllSources(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := InitialSidebarLinkDefaultsDocument()
	if len(doc.Items) == 0 {
		t.Fatal("expected source defaults")
	}
	for _, item := range doc.Items {
		if !item.Enabled {
			t.Fatalf("source %q should be enabled by default", item.Link.SourceName)
		}
		if !users.IsSourceSidebarCategory(item.Link.Category) {
			t.Fatalf("expected source category, got %q", item.Link.Category)
		}
	}
}

func TestEnsureAllSourcesInDefaults_addsMissingSourcesEnabled(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := SidebarLinkDefaultsDocument{Items: []SidebarLinkDefaultItem{}}
	merged, changed := EnsureAllSourcesInDefaults(doc)
	if !changed || len(merged.Items) == 0 {
		t.Fatalf("got %#v changed=%v", merged, changed)
	}
	for _, item := range merged.Items {
		if !item.Enabled {
			t.Fatalf("new source %q should default to enabled", item.Link.SourceName)
		}
	}
}

func TestFrontendDefaultsDocument_usesSourceDisplayName(t *testing.T) {
	initDefaultsTestConfig(t)
	doc := SidebarLinkDefaultsDocument{
		Items: []SidebarLinkDefaultItem{
			{
				Enabled: true,
				Link: users.SidebarLink{
					Name:       "exclude",
					Category:   "source",
					Target:     "/",
					SourceName: "/stale/path/not/in/sourcemap",
				},
			},
		},
	}
	out := FrontendDefaultsDocument(doc)
	if len(out.Items) != 1 {
		t.Fatalf("got %#v", out.Items)
	}
	if out.Items[0].Link.SourceName != "exclude" {
		t.Fatalf("expected display name exclude via name fallback, got %q", out.Items[0].Link.SourceName)
	}
}

func TestNormalizeDefaultsDocument_storesSourcePath(t *testing.T) {
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
	out := NormalizeDefaultsDocument(doc)
	if len(out.Items) != 1 {
		t.Fatalf("got %#v", out.Items)
	}
	if out.Items[0].Link.SourceName == "exclude" {
		t.Fatalf("expected canonical path, got display name %q", out.Items[0].Link.SourceName)
	}
}

func TestDocumentWithAllSources_preservesDistinctSourceTargets(t *testing.T) {
	initDefaultsTestConfig(t)
	excludeSource, ok := users.ResolveSourceKey("exclude")
	if !ok {
		t.Fatal("exclude source not in test config")
	}
	doc := SidebarLinkDefaultsDocument{
		Items: []SidebarLinkDefaultItem{
			{
				Enabled: true,
				Link: users.SidebarLink{
					Name:       "exclude",
					Category:   "source",
					Target:     "/",
					SourceName: excludeSource.Path,
				},
			},
			{
				Enabled: true,
				Link: users.SidebarLink{
					Name:       "exclude subfolder",
					Category:   "source",
					Target:     "/myfolder",
					SourceName: excludeSource.Path,
				},
			},
		},
	}
	out := DocumentWithAllSources(doc)
	var excludeTargets []string
	for _, item := range out.Items {
		if !users.IsSourceSidebarCategory(item.Link.Category) {
			continue
		}
		source, ok := users.ResolveSourceKey(item.Link.SourceName)
		if !ok || source.Path != excludeSource.Path {
			continue
		}
		excludeTargets = append(excludeTargets, item.Link.Target)
	}
	if len(excludeTargets) != 2 {
		t.Fatalf("expected two exclude targets, got %#v", excludeTargets)
	}
}
