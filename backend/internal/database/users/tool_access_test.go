package users

import (
	"encoding/json"
	"testing"
)

func assertBoolMapEntries(t *testing.T, name string, got, want map[string]bool) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d (got=%v want=%v)", name, len(got), len(want), got, want)
	}
	for key, wantVal := range want {
		gotVal, ok := got[key]
		if !ok {
			t.Fatalf("%s missing key %q", name, key)
		}
		if gotVal != wantVal {
			t.Fatalf("%s[%q] = %v, want %v", name, key, gotVal, wantVal)
		}
	}
}

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

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["effectiveToolAccess"]; !ok {
		t.Fatalf("expected effectiveToolAccess key in JSON, got keys: %v", mapKeys(payload))
	}

	var decoded FrontendUser
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	assertBoolMapEntries(t, "effectiveToolAccess", decoded.EffectiveToolAccess, u.EffectiveToolAccess)
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

	var payload map[string]bool
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload) != len(original) {
		t.Fatalf("toolAccess JSON length = %d, want %d", len(payload), len(original))
	}

	var decoded ToolAccessMap
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded) != len(original) {
		t.Fatalf("toolAccess length = %d, want %d", len(decoded), len(original))
	}
	for id, want := range original {
		got, ok := decoded[id]
		if !ok {
			t.Fatalf("toolAccess missing key %q", id)
		}
		if got != want {
			t.Fatalf("toolAccess[%q] = %v, want %v", id, got, want)
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

	wantToolAccess := ToolAccessMap{
		ToolSizeViewer:  true,
		ToolFileWatcher: false,
	}
	u := User{
		FrontendUser: FrontendUser{
			Username:   "bob",
			ToolAccess: wantToolAccess,
		},
	}

	encoded, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if _, ok := payload["toolAccess"]; !ok {
		t.Fatalf("expected toolAccess key in JSON, got keys: %v", mapKeys(payload))
	}

	var decoded User
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.ToolAccess) != len(wantToolAccess) {
		t.Fatalf("toolAccess length = %d, want %d", len(decoded.ToolAccess), len(wantToolAccess))
	}
	for id, want := range wantToolAccess {
		got, ok := decoded.ToolAccess[id]
		if !ok {
			t.Fatalf("toolAccess missing key %q", id)
		}
		if got != want {
			t.Fatalf("toolAccess[%q] = %v, want %v", id, got, want)
		}
	}
}

func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}
