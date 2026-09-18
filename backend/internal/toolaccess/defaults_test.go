package toolaccess

import (
	"encoding/json"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func testUser(toolAccess users.ToolAccessMap, admin bool) *users.User {
	u := &users.User{
		ToolAccess: toolAccess,
	}
	u.Permissions.Admin = admin
	return u
}

func testDoc(items []ToolAccessDefaultItem) ToolAccessDefaultsDocument {
	return ToolAccessDefaultsDocument{Items: items}
}

func TestHasToolAccessDefaultEnabled(t *testing.T) {
	doc := testDoc([]ToolAccessDefaultItem{
		{ToolID: users.ToolFileWatcher, Enabled: false, Enforced: false},
	})
	if HasToolAccess(testUser(nil, false), users.ToolFileWatcher, doc) {
		t.Fatal("expected fileWatcher disabled by default")
	}
}

func TestHasToolAccessUserOverride(t *testing.T) {
	doc := testDoc([]ToolAccessDefaultItem{
		{ToolID: users.ToolFileWatcher, Enabled: false, Enforced: false},
	})
	u := testUser(users.ToolAccessMap{users.ToolFileWatcher: true}, false)
	if !HasToolAccess(u, users.ToolFileWatcher, doc) {
		t.Fatal("expected user override to grant access")
	}
}

func TestHasToolAccessEnforced(t *testing.T) {
	doc := testDoc([]ToolAccessDefaultItem{
		{ToolID: users.ToolFileWatcher, Enabled: false, Enforced: true},
	})
	u := testUser(users.ToolAccessMap{users.ToolFileWatcher: true}, false)
	if HasToolAccess(u, users.ToolFileWatcher, doc) {
		t.Fatal("expected enforced default to deny access")
	}
}

func TestHasToolAccessAdminBypass(t *testing.T) {
	doc := testDoc([]ToolAccessDefaultItem{
		{ToolID: users.ToolFileWatcher, Enabled: false, Enforced: true},
	})
	u := testUser(users.ToolAccessMap{users.ToolFileWatcher: false}, true)
	if !HasToolAccess(u, users.ToolFileWatcher, doc) {
		t.Fatal("expected admin bypass")
	}
}

func TestValidateEnforcedToolAccess(t *testing.T) {
	doc := testDoc([]ToolAccessDefaultItem{
		{ToolID: users.ToolActivityViewer, Enabled: true, Enforced: true},
	})
	u := testUser(users.ToolAccessMap{users.ToolActivityViewer: false}, false)
	if err := ValidateEnforcedToolAccess(u, doc); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMergeDefaultToolAccess(t *testing.T) {
	doc := testDoc([]ToolAccessDefaultItem{
		{ToolID: users.ToolSizeViewer, Enabled: true, Enforced: false},
		{ToolID: users.ToolFileWatcher, Enabled: false, Enforced: false},
	})
	u := testUser(nil, false)
	if !MergeDefaultToolAccess(u, doc) {
		t.Fatal("expected change")
	}
	if !u.ToolAccess[users.ToolSizeViewer] || u.ToolAccess[users.ToolFileWatcher] {
		t.Fatalf("unexpected tool access: %+v", u.ToolAccess)
	}
}

func TestMergeEnforcedToolAccess(t *testing.T) {
	doc := testDoc([]ToolAccessDefaultItem{
		{ToolID: users.ToolDuplicateFinder, Enabled: false, Enforced: true},
	})
	u := testUser(users.ToolAccessMap{users.ToolDuplicateFinder: true}, false)
	if !MergeEnforcedToolAccess(u, doc) {
		t.Fatal("expected change")
	}
	if u.ToolAccess[users.ToolDuplicateFinder] {
		t.Fatal("expected enforced false")
	}
}

func TestToolAccessMapJSON(t *testing.T) {
	raw := `{"sizeViewer":true,"fileWatcher":false}`
	var m users.ToolAccessMap
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if !m[users.ToolSizeViewer] || m[users.ToolFileWatcher] {
		t.Fatalf("unexpected map: %+v", m)
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var round users.ToolAccessMap
	if err := json.Unmarshal(encoded, &round); err != nil {
		t.Fatal(err)
	}
	if round[users.ToolSizeViewer] != m[users.ToolSizeViewer] {
		t.Fatalf("round trip failed: %+v", round)
	}
}

func TestParseToolIDRejectsUnknown(t *testing.T) {
	if _, err := users.ParseToolID("notATool"); err == nil {
		t.Fatal("expected error for unknown tool")
	}
}
