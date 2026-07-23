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

// ErrEnforcedUserValueMismatch is returned when a user profile field differs from the enforced default value.
type ErrEnforcedUserValueMismatch struct {
	Path string
}

func (e ErrEnforcedUserValueMismatch) Error() string {
	return "user value does not match enforced default: " + e.Path
}

// ValidateUserAgainstEnforcedDefaults rejects users whose enforced fields differ from universal defaults.
func ValidateUserAgainstEnforcedDefaults(u *users.User, defaults UserDefaults, enforced UserDefaultsEnforcement) error {
	if !EnforcementAppliesToUser(u) {
		return nil
	}
	paths := withoutAuthManagedEnforcementPaths(EnforcedPathSet(enforced))
	if len(paths) == 0 || u == nil || u.Username == "" || u.Username == users.AnonymousUserName {
		return nil
	}
	userBytes, err := json.Marshal(ProfileFromUser(u))
	if err != nil {
		return fmt.Errorf("marshal user profile: %w", err)
	}
	defBytes, err := json.Marshal(ProfileFromUserDefaults(defaults))
	if err != nil {
		return fmt.Errorf("marshal default profile: %w", err)
	}
	var userMap, defMap map[string]interface{}
	if err := json.Unmarshal(userBytes, &userMap); err != nil {
		return fmt.Errorf("parse user profile: %w", err)
	}
	if err := json.Unmarshal(defBytes, &defMap); err != nil {
		return fmt.Errorf("parse default profile: %w", err)
	}
	for path := range paths {
		expected, ok := valueAtJSONPath(defMap, path)
		if !ok {
			continue
		}
		actual, ok := valueAtJSONPath(userMap, path)
		if !ok || !jsonValuesEqual(expected, actual) {
			return ErrEnforcedUserValueMismatch{Path: path}
		}
	}
	return nil
}

func jsonValuesEqual(a, b interface{}) bool {
	ab, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ab) == string(bb)
}

// ApplyEnforcedDefaultsFrom merges enforced default paths from d onto u's profile.
func ApplyEnforcedDefaultsFrom(u *users.User, d UserDefaults, e UserDefaultsEnforcement) {
	if !EnforcementAppliesToUser(u) {
		return
	}
	if u == nil || u.Username == "anonymous" {
		return
	}
	paths := withoutAuthManagedEnforcementPaths(EnforcedPathSet(e))
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
	if !EnforcementAppliesToUser(u) {
		return false
	}
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
