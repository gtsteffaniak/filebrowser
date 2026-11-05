package settings

import (
	"encoding/json"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// ProfilePathsFromUserJSON returns the profile dot-paths whose flat user JSON keys are
// present in raw (a create request "data" object). Keys inside "preview" and
// "fileLoading" are mapped individually. Permissions are tracked separately.
func ProfilePathsFromUserJSON(raw []byte) map[string]struct{} {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || len(top) == 0 {
		return nil
	}
	paths := make(map[string]struct{})
	for key, val := range top {
		if path, ok := userJSONFieldEnforcementPaths[key]; ok {
			paths[path] = struct{}{}
			continue
		}
		var sub map[string]string
		switch key {
		case "preview":
			sub = previewJSONSubfields
		case "fileLoading":
			sub = fileLoadingJSONSubfields
		default:
			continue
		}
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(val, &nested); err != nil {
			continue
		}
		for subKey := range nested {
			if path, ok := sub[subKey]; ok {
				paths[path] = struct{}{}
			}
		}
	}
	if len(paths) == 0 {
		return nil
	}
	return paths
}

// ApplyProfilePathsFrom copies the listed profile dot-paths from source onto u,
// leaving every other profile field as it is.
func ApplyProfilePathsFrom(u *users.User, source UserProfile, paths map[string]struct{}) {
	if u == nil || len(paths) == 0 {
		return
	}
	currentBytes, err := ProfileJSONFromUser(u)
	if err != nil {
		return
	}
	patchBytes, err := profilePatchJSONForPaths(source, paths)
	if err != nil {
		return
	}
	mergedBytes, err := mergeUserDefaultsJSON(currentBytes, patchBytes)
	if err != nil {
		return
	}
	_ = ApplyProfileToUser(u, mergedBytes)
}
