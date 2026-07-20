package settings

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// ApplyFullProfileFromDefaults replaces u profile fields from defaults template.
func ApplyFullProfileFromDefaults(u *users.User, d UserDefaults) {
	if u == nil || u.Username == "anonymous" {
		return
	}
	ExpandProfileIntoUser(u, ProfileFromUserDefaults(d))
}

// ApplyEnforcedDefaultsFrom merges enforced default paths from d onto u's profile.
func ApplyEnforcedDefaultsFrom(u *users.User, d UserDefaults, e UserDefaultsEnforcement) {
	if u == nil || u.Username == "anonymous" {
		return
	}
	paths := EnforcedPathSet(e)
	if len(paths) == 0 {
		return
	}
	currentBytes, err := ProfileJSONFromUser(u)
	if err != nil {
		return
	}
	patchBytes, err := profilePatchJSONForPaths(ProfileFromUserDefaults(d), paths)
	if err != nil {
		return
	}
	mergedBytes, err := mergeUserDefaultsJSON(currentBytes, patchBytes)
	if err != nil {
		return
	}
	_ = ApplyProfileToUser(u, mergedBytes)
}

// SyncEnforcedDefaultsOntoUser applies enforced defaults onto u. Returns true if profile changed.
func SyncEnforcedDefaultsOntoUser(u *users.User, d UserDefaults, e UserDefaultsEnforcement) bool {
	if u == nil || u.Username == "anonymous" {
		return false
	}
	if len(EnforcedPathSet(e)) == 0 {
		return false
	}
	before, err := ProfileJSONFromUser(u)
	if err != nil {
		return false
	}
	ApplyEnforcedDefaultsFrom(u, d, e)
	after, err := ProfileJSONFromUser(u)
	if err != nil {
		return false
	}
	return string(before) != string(after)
}

func profilePatchJSONForPaths(source UserProfile, paths map[string]struct{}) ([]byte, error) {
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

func profilePatchForPaths(source UserProfile, paths map[string]struct{}) (UserProfile, error) {
	patchBytes, err := profilePatchJSONForPaths(source, paths)
	if err != nil {
		return UserProfile{}, err
	}
	var patch UserProfile
	if err := json.Unmarshal(patchBytes, &patch); err != nil {
		return UserProfile{}, fmt.Errorf("unmarshal profile patch: %w", err)
	}
	return patch, nil
}

func valueAtJSONPath(root map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
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
	parts := strings.Split(path, ".")
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
