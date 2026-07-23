package settings

import (
	"encoding/json"
	"reflect"
	"strings"
)

// AuthManagedEnforcementPaths are default paths owned by authentication (adminGroup,
// AdminUsername) rather than the user-defaults template. Enforcement must not block
// or overwrite auth-granted admin privileges.
var AuthManagedEnforcementPaths = map[string]struct{}{
	"account.permissions.admin": {},
}

func withoutAuthManagedEnforcementPaths(paths map[string]struct{}) map[string]struct{} {
	if len(paths) == 0 {
		return paths
	}
	out := make(map[string]struct{}, len(paths))
	for path := range paths {
		if _, authManaged := AuthManagedEnforcementPaths[path]; authManaged {
			continue
		}
		out[path] = struct{}{}
	}
	return out
}

// EnforcedPathSet returns dot-paths with enforcement enabled.
func EnforcedPathSet(e UserDefaultsEnforcement) map[string]struct{} {
	var paths []string
	collectEnforcedPaths(reflect.ValueOf(e), "", &paths)
	out := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		out[p] = struct{}{}
	}
	return out
}

// EnforcedFieldPaths returns dot-paths for enforced settings (settings API / future Profile UI).
func EnforcedFieldPaths(e UserDefaultsEnforcement) []string {
	return collectEnforcedPathsList(reflect.ValueOf(e), "")
}

func collectEnforcedPathsList(v reflect.Value, prefix string) []string {
	var paths []string
	collectEnforcedPaths(v, prefix, &paths)
	return paths
}

func collectEnforcedPaths(v reflect.Value, prefix string, out *[]string) {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		jsonTag := field.Tag.Get("json")
		name := strings.Split(jsonTag, ",")[0]
		if name == "" || name == "-" {
			name = strings.ToLower(field.Name[:1]) + field.Name[1:]
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.Bool:
			if fv.Bool() {
				*out = append(*out, path)
			}
		case reflect.Struct:
			collectEnforcedPaths(fv, path, out)
		}
	}
}

// MergeEnforcedPatchJSON merges a partial JSON patch into base enforcement.
func MergeEnforcedPatchJSON(base UserDefaultsEnforcement, patchJSON []byte) (UserDefaultsEnforcement, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return UserDefaultsEnforcement{}, err
	}
	mergedBytes, err := mergeUserDefaultsJSON(baseBytes, patchJSON)
	if err != nil {
		return UserDefaultsEnforcement{}, err
	}
	var merged UserDefaultsEnforcement
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return UserDefaultsEnforcement{}, err
	}
	return merged, nil
}

func MergeEnforcedPatchJSONBytes(baseJSON, patchJSON []byte) ([]byte, error) {
	return mergeUserDefaultsJSON(baseJSON, patchJSON)
}
