package users

import (
	"encoding/json"
	"fmt"
)

// ToolID identifies a built-in application tool (matches frontend route segments).
type ToolID string

const (
	ToolSizeViewer         ToolID = "sizeViewer"
	ToolDuplicateFinder    ToolID = "duplicateFinder"
	ToolAdvancedSearch     ToolID = "advancedSearch"
	ToolMaterialIconPicker ToolID = "materialIconPicker"
	ToolFileWatcher        ToolID = "fileWatcher"
	ToolActivityViewer     ToolID = "activityViewer"
)

// AllToolIDs lists every defined tool in stable catalog order.
var AllToolIDs = []ToolID{
	ToolSizeViewer,
	ToolDuplicateFinder,
	ToolAdvancedSearch,
	ToolMaterialIconPicker,
	ToolFileWatcher,
	ToolActivityViewer,
}

// Valid reports whether id is a known tool constant.
func (id ToolID) Valid() bool {
	switch id {
	case ToolSizeViewer, ToolDuplicateFinder, ToolAdvancedSearch,
		ToolMaterialIconPicker, ToolFileWatcher, ToolActivityViewer:
		return true
	default:
		return false
	}
}

// ParseToolID parses s into a ToolID when it names a catalog tool.
func ParseToolID(s string) (ToolID, error) {
	id := ToolID(s)
	if !id.Valid() {
		return "", fmt.Errorf("unknown tool id %q", s)
	}
	return id, nil
}

// ToolAccessMap stores per-user tool grants keyed by ToolID.
// JSON uses string keys for API compatibility.
type ToolAccessMap map[ToolID]bool

// MarshalJSON encodes the map with string keys.
func (m ToolAccessMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	out := make(map[string]bool, len(m))
	for id, granted := range m {
		if id.Valid() {
			out[string(id)] = granted
		}
	}
	return json.Marshal(out)
}

// UnmarshalJSON decodes string-keyed JSON into typed ToolID keys.
func (m *ToolAccessMap) UnmarshalJSON(data []byte) error {
	if m == nil {
		return fmt.Errorf("ToolAccessMap: UnmarshalJSON on nil pointer")
	}
	if string(data) == "null" {
		*m = nil
		return nil
	}
	var raw map[string]bool
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := make(ToolAccessMap, len(raw))
	for key, granted := range raw {
		id, err := ParseToolID(key)
		if err != nil {
			return err
		}
		out[id] = granted
	}
	*m = out
	return nil
}
