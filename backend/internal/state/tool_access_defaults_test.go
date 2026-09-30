package state

import (
	"errors"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/toolaccess"
)

func TestPatchToolAccessDefaults_retriesPendingResync(t *testing.T) {
	initSidebarLinkTestDB(t)

	doc := toolaccess.InitialToolAccessDefaultsDocument()
	if err := PatchToolAccessDefaults(doc); err != nil {
		t.Fatal(err)
	}

	toolAccessDefaultsResyncPending = true
	if err := PatchToolAccessDefaults(doc); err != nil {
		t.Fatal(err)
	}
	if toolAccessDefaultsResyncPending {
		t.Fatal("expected pending resync to clear after successful retry")
	}
}

func TestPatchToolAccessDefaults_marksPendingOnResyncFailure(t *testing.T) {
	initSidebarLinkTestDB(t)

	errInjectResyncToolAccessDefaults = errors.New("simulated resync failure")
	t.Cleanup(func() {
		errInjectResyncToolAccessDefaults = nil
		toolAccessDefaultsResyncPending = false
	})

	doc := toolaccess.InitialToolAccessDefaultsDocument()
	doc.Items = append(doc.Items, toolaccess.ToolAccessDefaultItem{
		ToolID:   users.ToolFileWatcher,
		Enabled:  false,
		Enforced: true,
	})
	if err := PatchToolAccessDefaults(doc); err == nil {
		t.Fatal("expected resync failure")
	}
	if !toolAccessDefaultsResyncPending {
		t.Fatal("expected pending resync after failure")
	}

	errInjectResyncToolAccessDefaults = nil
	if err := PatchToolAccessDefaults(doc); err != nil {
		t.Fatal(err)
	}
	if toolAccessDefaultsResyncPending {
		t.Fatal("expected pending resync to clear after successful retry")
	}
}

func TestResyncToolAccessDefaultsForAllUsers_appliesDefaults(t *testing.T) {
	initSidebarLinkTestDB(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "resync-user",
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	doc := EffectiveToolAccessDefaults()
	for i := range doc.Items {
		if doc.Items[i].ToolID == users.ToolFileWatcher {
			doc.Items[i].Enabled = false
			doc.Items[i].Enforced = false
			break
		}
	}
	toolAccessDefaultsMu.Lock()
	toolAccessDefaults = doc
	toolAccessDefaultsMu.Unlock()

	loaded, err := GetUserByUsername("resync-user")
	if err != nil {
		t.Fatal(err)
	}
	loaded.ToolAccess = users.ToolAccessMap{}
	if err = UpdateUser(&loaded, "", "toolAccess"); err != nil {
		t.Fatal(err)
	}

	if err = ResyncToolAccessDefaultsForAllUsers(); err != nil {
		t.Fatal(err)
	}

	loaded, err = GetUserByUsername("resync-user")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ToolAccess[users.ToolFileWatcher] {
		t.Fatal("expected fileWatcher denied after resync")
	}
}
