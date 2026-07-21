package settings

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// CollectMapLeafPaths returns dot-paths for explicitly set leaves in a nested map.
func CollectMapLeafPaths(raw map[string]interface{}, prefix string) []string {
	if raw == nil {
		return nil
	}
	var paths []string
	collectMapLeafPaths(raw, prefix, &paths)
	return paths
}

func collectMapLeafPaths(raw map[string]interface{}, prefix string, out *[]string) {
	for key, val := range raw {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		child, ok := val.(map[string]interface{})
		if ok && len(child) > 0 {
			collectMapLeafPaths(child, path, out)
			continue
		}
		if ok && len(child) == 0 {
			*out = append(*out, path)
			continue
		}
		*out = append(*out, path)
	}
}

// ConfigSpecifiedUserDefaultPaths returns dot-paths explicitly set in config userDefaults.
func ConfigSpecifiedUserDefaultPaths() []string {
	if len(Env.ConfigUserDefaultsSpecifiedPaths) == 0 {
		return nil
	}
	out := make([]string, len(Env.ConfigUserDefaultsSpecifiedPaths))
	copy(out, Env.ConfigUserDefaultsSpecifiedPaths)
	return out
}

// ConfigSpecifiedUserDefaultPathSet returns a set of config-specified user default paths.
func ConfigSpecifiedUserDefaultPathSet() map[string]struct{} {
	paths := Env.ConfigUserDefaultsSpecifiedPaths
	out := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		out[p] = struct{}{}
	}
	return out
}

// IsUserDefaultLockedFromConfig reports whether path was explicitly set in config userDefaults.
func IsUserDefaultLockedFromConfig(path string) bool {
	_, ok := ConfigSpecifiedUserDefaultPathSet()[path]
	return ok
}

// ApplyEnforcementFromPaths sets enforcement flags for config-specified paths.
func ApplyEnforcementFromPaths(enforced *UserDefaultsEnforcement, paths []string) {
	if enforced == nil || len(paths) == 0 {
		return
	}
	pathSet := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		pathSet[p] = struct{}{}
	}
	for path := range pathSet {
		_ = setEnforcementBoolAtPath(enforced, path, true)
	}
}

func setEnforcementBoolAtPath(enforced *UserDefaultsEnforcement, path string, value bool) error {
	if !value {
		return nil
	}
	parts := strings.Split(path, ".")
	v := reflect.ValueOf(enforced).Elem()
	for i, part := range parts {
		if v.Kind() != reflect.Struct {
			return fmt.Errorf("invalid enforcement path %q", path)
		}
		fieldIdx, ok := structFieldIndexByJSONTag(v, part)
		if !ok {
			return fmt.Errorf("unknown enforcement path %q", path)
		}
		v = v.Field(fieldIdx)
		if i == len(parts)-1 {
			if v.Kind() != reflect.Bool {
				return fmt.Errorf("enforcement path %q is not a bool field", path)
			}
			v.SetBool(true)
			return nil
		}
	}
	return fmt.Errorf("invalid enforcement path %q", path)
}

func structFieldIndexByJSONTag(v reflect.Value, name string) (int, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		jsonTag := field.Tag.Get("json")
		tagName := strings.Split(jsonTag, ",")[0]
		if tagName == "" || tagName == "-" {
			tagName = strings.ToLower(field.Name[:1]) + field.Name[1:]
		}
		if tagName == name {
			return i, true
		}
	}
	return 0, false
}

// CollectJSONPatchLeafPaths returns dot-paths updated by a JSON patch object.
func CollectJSONPatchLeafPaths(patchJSON []byte) ([]string, error) {
	if len(patchJSON) == 0 || string(patchJSON) == "{}" || string(patchJSON) == "null" {
		return nil, fmt.Errorf("empty user defaults patch")
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(patchJSON, &raw); err != nil {
		return nil, fmt.Errorf("invalid patch JSON: %w", err)
	}
	return CollectMapLeafPaths(raw, ""), nil
}

// ValidateUserDefaultsPatchNotConfigLocked rejects patches touching config-locked paths.
func ValidateUserDefaultsPatchNotConfigLocked(patchJSON []byte) error {
	if !Env.ConfigUserDefaultsSpecified {
		return nil
	}
	paths, err := CollectJSONPatchLeafPaths(patchJSON)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if IsUserDefaultLockedFromConfig(path) {
			return fmt.Errorf("user default %q is locked from config file", path)
		}
	}
	return nil
}
