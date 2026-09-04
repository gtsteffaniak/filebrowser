package sharedefaults

import (
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// ApplyDefaultsToEditable merges default template values into editable for new shares.
func ApplyDefaultsToEditable(editable *share.ShareEditable, defaults settings.ShareDefaults) {
	if editable == nil {
		return
	}
	current := EditableToDefaults(*editable)
	currentBytes, err := json.Marshal(current)
	if err != nil {
		return
	}
	defaultBytes, err := json.Marshal(defaults)
	if err != nil {
		return
	}
	mergedBytes, err := mergeJSON(defaultBytes, currentBytes)
	if err != nil {
		return
	}
	var merged settings.ShareDefaults
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return
	}
	*editable = DefaultsToEditable(merged)
}

// ApplyEnforcedDefaults overwrites enforced fields on editable from the defaults template.
func ApplyEnforcedDefaults(editable *share.ShareEditable, defaults settings.ShareDefaults, enforced settings.ShareDefaultsEnforcement) {
	if editable == nil {
		return
	}
	paths := settings.ShareEnforcedPathSet(enforced)
	if len(paths) == 0 {
		return
	}
	currentBytes, err := json.Marshal(EditableToDefaults(*editable))
	if err != nil {
		return
	}
	patchBytes, err := patchJSONForPaths(defaults, paths)
	if err != nil {
		return
	}
	mergedBytes, err := mergeJSON(currentBytes, patchBytes)
	if err != nil {
		return
	}
	var merged settings.ShareDefaults
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return
	}
	*editable = DefaultsToEditable(merged)
}

// ValidateEditableNotEnforced rejects requests where enforced fields differ from the defaults template.
func ValidateEditableNotEnforced(after *share.ShareEditable, enforced settings.ShareDefaultsEnforcement, defaults settings.ShareDefaults) error {
	if after == nil {
		return nil
	}
	paths := settings.ShareEnforcedPathSet(enforced)
	if len(paths) == 0 {
		return nil
	}
	afterBytes, err := json.Marshal(EditableToDefaults(*after))
	if err != nil {
		return fmt.Errorf("marshal share editable: %w", err)
	}
	defBytes, err := json.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("marshal share defaults: %w", err)
	}
	var afterMap, defMap map[string]interface{}
	if err := json.Unmarshal(afterBytes, &afterMap); err != nil {
		return fmt.Errorf("parse share editable: %w", err)
	}
	if err := json.Unmarshal(defBytes, &defMap); err != nil {
		return fmt.Errorf("parse share defaults: %w", err)
	}
	for path := range paths {
		expected, ok := valueAtJSONPath(defMap, path)
		if !ok {
			continue
		}
		actual, ok := valueAtJSONPath(afterMap, path)
		if !ok || !jsonValuesEqual(expected, actual) {
			return settings.ErrEnforcedShareValueMismatch{Path: path}
		}
	}
	return nil
}

func patchJSONForPaths(source settings.ShareDefaults, paths map[string]struct{}) ([]byte, error) {
	srcBytes, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	var srcMap map[string]interface{}
	if err := json.Unmarshal(srcBytes, &srcMap); err != nil {
		return nil, err
	}
	patchMap := make(map[string]interface{})
	for path := range paths {
		if val, ok := valueAtJSONPath(srcMap, path); ok {
			setAtJSONPath(patchMap, path, val)
		}
	}
	return json.Marshal(patchMap)
}

func mergeJSON(baseJSON, patchJSON []byte) ([]byte, error) {
	var base map[string]interface{}
	if err := json.Unmarshal(baseJSON, &base); err != nil {
		return nil, err
	}
	var patch map[string]interface{}
	if err := json.Unmarshal(patchJSON, &patch); err != nil {
		return nil, err
	}
	deepMergeMaps(base, patch)
	return json.Marshal(base)
}

func deepMergeMaps(base, patch map[string]interface{}) {
	for key, patchVal := range patch {
		baseVal, exists := base[key]
		if !exists {
			base[key] = patchVal
			continue
		}
		baseMap, baseOK := baseVal.(map[string]interface{})
		patchMap, patchOK := patchVal.(map[string]interface{})
		if baseOK && patchOK {
			deepMergeMaps(baseMap, patchMap)
			continue
		}
		base[key] = patchVal
	}
}

func valueAtJSONPath(root map[string]interface{}, path string) (interface{}, bool) {
	parts := splitPath(path)
	var cur interface{} = root
	for _, part := range parts {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		val, exists := m[part]
		if !exists {
			return nil, false
		}
		cur = val
	}
	return cur, true
}

func setAtJSONPath(root map[string]interface{}, path string, value interface{}) {
	parts := splitPath(path)
	cur := root
	for i, part := range parts {
		if i == len(parts)-1 {
			cur[part] = value
			return
		}
		next, ok := cur[part].(map[string]interface{})
		if !ok {
			next = make(map[string]interface{})
			cur[part] = next
		}
		cur = next
	}
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	out := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] == '.' {
			if i > start {
				out = append(out, path[start:i])
			}
			start = i + 1
		}
	}
	if start < len(path) {
		out = append(out, path[start:])
	}
	return out
}

func jsonValuesEqual(a, b interface{}) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}
