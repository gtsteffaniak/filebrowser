package settings

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// ErrEnforcedShareValueMismatch is returned when a share field differs from the enforced default value.
type ErrEnforcedShareValueMismatch struct {
	Path string
}

func (e ErrEnforcedShareValueMismatch) Error() string {
	return "share value does not match enforced default: " + e.Path
}

// ShareEnforcedPathSet returns dot-paths with enforcement enabled.
func ShareEnforcedPathSet(e ShareDefaultsEnforcement) map[string]struct{} {
	var paths []string
	collectEnforcedPaths(reflect.ValueOf(e), "", &paths)
	out := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		out[p] = struct{}{}
	}
	return out
}

// MergeShareDefaultsPatchJSON merges a partial JSON patch into base share defaults.
func MergeShareDefaultsPatchJSON(base ShareDefaults, patchJSON []byte) (ShareDefaults, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return ShareDefaults{}, err
	}
	mergedBytes, err := mergeUserDefaultsJSON(baseBytes, patchJSON)
	if err != nil {
		return ShareDefaults{}, err
	}
	var merged ShareDefaults
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return ShareDefaults{}, fmt.Errorf("unmarshal merged share defaults: %w", err)
	}
	return merged, nil
}

// MergeShareEnforcedPatchJSON merges a partial JSON patch into base share enforcement.
func MergeShareEnforcedPatchJSON(base ShareDefaultsEnforcement, patchJSON []byte) (ShareDefaultsEnforcement, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return ShareDefaultsEnforcement{}, err
	}
	mergedBytes, err := mergeUserDefaultsJSON(baseBytes, patchJSON)
	if err != nil {
		return ShareDefaultsEnforcement{}, err
	}
	var merged ShareDefaultsEnforcement
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return ShareDefaultsEnforcement{}, fmt.Errorf("unmarshal merged share enforcement: %w", err)
	}
	return merged, nil
}
