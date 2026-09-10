package users

import (
	"encoding/json"
	"testing"
)

func TestFrontendUserEffectiveToolAccessJSON(t *testing.T) {
	t.Parallel()

	u := FrontendUser{
		Username: "alice",
		EffectiveToolAccess: map[string]bool{
			"sizeViewer":      true,
			"fileWatcher":     false,
			"activityViewer":  true,
			"duplicateFinder": false,
		},
	}

	encoded, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	var decoded FrontendUser
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	for toolID, granted := range u.EffectiveToolAccess {
		if decoded.EffectiveToolAccess[toolID] != granted {
			t.Fatalf("effectiveToolAccess[%q] = %v, want %v", toolID, decoded.EffectiveToolAccess[toolID], granted)
		}
	}
}

func TestToolAccessMapRoundTripValidIDs(t *testing.T) {
	t.Parallel()

	original := ToolAccessMap{
		ToolSizeViewer:         true,
		ToolFileWatcher:        false,
		ToolActivityViewer:     true,
		ToolDuplicateFinder:    false,
		ToolAdvancedSearch:     true,
		ToolMaterialIconPicker: false,
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ToolAccessMap
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	for id, granted := range original {
		if decoded[id] != granted {
			t.Fatalf("toolAccess[%q] = %v, want %v", id, decoded[id], granted)
		}
	}
}

func TestToolAccessMapRejectsUnknownToolID(t *testing.T) {
	t.Parallel()

	var m ToolAccessMap
	err := json.Unmarshal([]byte(`{"sizeViewer":true,"notATool":false}`), &m)
	if err == nil {
		t.Fatal("expected unknown tool id to be rejected")
	}
}

func TestUserToolAccessJSONRoundTrip(t *testing.T) {
	t.Parallel()

	u := User{
		FrontendUser: FrontendUser{
			Username: "bob",
			ToolAccess: ToolAccessMap{
				ToolSizeViewer:  true,
				ToolFileWatcher: false,
			},
		},
	}

	encoded, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	var decoded User
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	if !decoded.ToolAccess[ToolSizeViewer] {
		t.Fatal("expected sizeViewer to remain enabled")
	}
	if decoded.ToolAccess[ToolFileWatcher] {
		t.Fatal("expected fileWatcher to remain disabled")
	}
}
