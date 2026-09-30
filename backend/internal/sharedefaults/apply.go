package sharedefaults

import (
	"encoding/json"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// NormalizeUploadShareEditable ensures upload shares always request create permission.
func NormalizeUploadShareEditable(editable *share.ShareEditable) {
	if editable != nil && editable.ShareType == "upload" {
		editable.AllowCreate = true
	}
}

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
	afterDefaults := EditableToDefaults(*after)
	for path := range paths {
		expected, ok := settings.ShareDefaultsValueAtPath(defaults, path)
		if !ok {
			continue
		}
		actual, ok := settings.ShareDefaultsValueAtPath(afterDefaults, path)
		if !ok || !jsonValuesEqual(expected, actual) {
			return settings.ErrEnforcedShareValueMismatch{Path: path}
		}
	}
	return nil
}

func patchJSONForPaths(source settings.ShareDefaults, paths map[string]struct{}) ([]byte, error) {
	patchMap := make(map[string]interface{}, len(paths))
	for path := range paths {
		val, ok := settings.ShareDefaultsValueAtPath(source, path)
		if !ok {
			continue
		}
		patchMap[path] = val
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

func jsonValuesEqual(a, b interface{}) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}
