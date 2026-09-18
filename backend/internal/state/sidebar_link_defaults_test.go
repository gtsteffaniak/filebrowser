package state

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
)

func TestPatchSidebarLinkDefaults_retriesPendingResync(t *testing.T) {
	initSidebarLinkTestDB(t)

	doc := usersidebar.InitialSidebarLinkDefaultsDocument()
	doc.Items = append(doc.Items, usersidebar.SidebarLinkDefaultItem{
		Enabled: true,
		Link: users.SidebarLink{
			Name:     "Wiki",
			Category: "custom",
			Target:   "/wiki",
			Icon:     "link",
		},
	})
	if err := PatchSidebarLinkDefaults(doc); err != nil {
		t.Fatal(err)
	}

	sidebarLinkDefaultsResyncPending = true
	if err := PatchSidebarLinkDefaults(doc); err != nil {
		t.Fatal(err)
	}
	if sidebarLinkDefaultsResyncPending {
		t.Fatal("expected pending resync to clear after successful retry")
	}
}

func TestPatchSidebarLinkDefaults_marksPendingOnResyncFailure(t *testing.T) {
	initSidebarLinkTestDB(t)

	errInjectResyncSidebarDefaults = errors.New("simulated resync failure")
	t.Cleanup(func() {
		errInjectResyncSidebarDefaults = nil
		sidebarLinkDefaultsResyncPending = false
	})

	doc := usersidebar.InitialSidebarLinkDefaultsDocument()
	doc.Items = append(doc.Items, usersidebar.SidebarLinkDefaultItem{
		Enabled: true,
		Link: users.SidebarLink{
			Name:     "Docs",
			Category: "custom",
			Target:   "/docs",
		},
	})
	if err := PatchSidebarLinkDefaults(doc); err == nil {
		t.Fatal("expected resync failure")
	}
	if !sidebarLinkDefaultsResyncPending {
		t.Fatal("expected pending resync after failure")
	}

	errInjectResyncSidebarDefaults = nil
	if err := PatchSidebarLinkDefaults(doc); err != nil {
		t.Fatal(err)
	}
	if sidebarLinkDefaultsResyncPending {
		t.Fatal("expected pending resync to clear after successful retry")
	}
}
