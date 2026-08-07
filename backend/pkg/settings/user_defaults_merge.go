package settings

import (
	"encoding/json"
	"fmt"
)

// MergeUserDefaults returns a copy of base with patch merged on top (JSON deep merge).
// Both values are treated as complete documents (e.g. loaded from the database).
func MergeUserDefaults(base, patch UserDefaults) (UserDefaults, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return UserDefaults{}, fmt.Errorf("marshal base user defaults: %w", err)
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return UserDefaults{}, fmt.Errorf("marshal patch user defaults: %w", err)
	}
	mergedBytes, err := mergeUserDefaultsJSON(baseBytes, patchBytes)
	if err != nil {
		return UserDefaults{}, err
	}
	var merged UserDefaults
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return UserDefaults{}, fmt.Errorf("unmarshal merged user defaults: %w", err)
	}
	return merged, nil
}

// MergeUserDefaultsPatchJSONBytes merges patchJSON into baseJSON (both JSON objects).
func MergeUserDefaultsPatchJSONBytes(baseJSON, patchJSON []byte) ([]byte, error) {
	return mergeUserDefaultsJSON(baseJSON, patchJSON)
}

// MergeUserDefaultsPatchJSON merges a partial JSON patch into base user defaults.
func MergeUserDefaultsPatchJSON(base UserDefaults, patchJSON []byte) (UserDefaults, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return UserDefaults{}, fmt.Errorf("marshal base user defaults: %w", err)
	}
	mergedBytes, err := mergeUserDefaultsJSON(baseBytes, patchJSON)
	if err != nil {
		return UserDefaults{}, err
	}
	var merged UserDefaults
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return UserDefaults{}, fmt.Errorf("unmarshal merged user defaults: %w", err)
	}
	return merged, nil
}

func mergeUserDefaultsJSON(baseJSON, patchJSON []byte) ([]byte, error) {
	var baseMap map[string]interface{}
	if err := json.Unmarshal(baseJSON, &baseMap); err != nil {
		return nil, fmt.Errorf("unmarshal base map: %w", err)
	}
	if baseMap == nil {
		baseMap = make(map[string]interface{})
	}
	var patchMap map[string]interface{}
	if err := json.Unmarshal(patchJSON, &patchMap); err != nil {
		return nil, fmt.Errorf("unmarshal patch map: %w", err)
	}
	if patchMap == nil {
		out, err := json.Marshal(baseMap)
		if err != nil {
			return nil, fmt.Errorf("marshal merged map: %w", err)
		}
		return out, nil
	}
	mergeJSONMaps(baseMap, patchMap)
	out, err := json.Marshal(baseMap)
	if err != nil {
		return nil, fmt.Errorf("marshal merged map: %w", err)
	}
	return out, nil
}

func mergeJSONMaps(base, patch map[string]interface{}) {
	for key, patchVal := range patch {
		baseVal, ok := base[key]
		if !ok {
			base[key] = patchVal
			continue
		}
		baseMap, baseOk := baseVal.(map[string]interface{})
		patchMap, patchOk := patchVal.(map[string]interface{})
		if baseOk && patchOk {
			mergeJSONMaps(baseMap, patchMap)
			base[key] = baseMap
			continue
		}
		base[key] = patchVal
	}
}

// CountJSONPatchLeaves counts scalar (and non-object) leaves in a JSON patch object.
func CountJSONPatchLeaves(v interface{}) int {
	if v == nil {
		return 0
	}
	switch t := v.(type) {
	case map[string]interface{}:
		n := 0
		for _, child := range t {
			n += CountJSONPatchLeaves(child)
		}
		return n
	default:
		return 1
	}
}

// ValidateSinglePropertyUserDefaultsPatch rejects patches that change more than one leaf value.
func ValidateSinglePropertyUserDefaultsPatch(patchJSON []byte) error {
	if len(patchJSON) == 0 || string(patchJSON) == "{}" || string(patchJSON) == "null" {
		return fmt.Errorf("empty user defaults patch")
	}
	var v interface{}
	if err := json.Unmarshal(patchJSON, &v); err != nil {
		return fmt.Errorf("invalid patch JSON: %w", err)
	}
	n := CountJSONPatchLeaves(v)
	if n != 1 {
		return fmt.Errorf("patch must update exactly one property (got %d)", n)
	}
	return nil
}
