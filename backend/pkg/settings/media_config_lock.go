package settings

import (
	"encoding/json"
	"fmt"
)

// MediaConfigLockMessage explains partial config locking of media settings in the admin UI.
const MediaConfigLockMessage = "some media settings were set in the config file and cannot be changed here until an admin removes them from the config file"

// MediaFieldConfigLockMessage formats a single-path lock error.
func MediaFieldConfigLockMessage(path string) string {
	return fmt.Sprintf("media setting %q is locked from config file", path)
}

// ConfigSpecifiedMediaPaths returns dot-paths explicitly set in config integrations.media.
func ConfigSpecifiedMediaPaths() []string {
	if len(Env.ConfigMediaSpecifiedPaths) == 0 {
		return nil
	}
	out := make([]string, len(Env.ConfigMediaSpecifiedPaths))
	copy(out, Env.ConfigMediaSpecifiedPaths)
	return out
}

// ConfigSpecifiedMediaPathSet returns a set of config-specified media paths.
func ConfigSpecifiedMediaPathSet() map[string]struct{} {
	paths := Env.ConfigMediaSpecifiedPaths
	out := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		out[p] = struct{}{}
	}
	return out
}

// IsMediaSettingLockedFromConfig reports whether path was explicitly set in config integrations.media.
func IsMediaSettingLockedFromConfig(path string) bool {
	_, ok := ConfigSpecifiedMediaPathSet()[path]
	return ok
}

// MediaSettingsLockedFromConfig reports whether any media settings were set in the config file.
func MediaSettingsLockedFromConfig() bool {
	return len(Env.ConfigMediaSpecifiedPaths) > 0
}

// MediaPatchJSONForPaths builds a partial JSON patch containing only the given dot-paths from source.
func MediaPatchJSONForPaths(source any, paths []string) ([]byte, error) {
	if len(paths) == 0 {
		return []byte("{}"), nil
	}
	srcBytes, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("marshal media settings: %w", err)
	}
	var srcMap map[string]interface{}
	if err := json.Unmarshal(srcBytes, &srcMap); err != nil {
		return nil, fmt.Errorf("unmarshal media settings map: %w", err)
	}
	patchMap := make(map[string]interface{})
	for _, path := range paths {
		if val, ok := valueAtJSONPath(srcMap, path); ok {
			setAtJSONPath(patchMap, path, val)
		}
	}
	return json.Marshal(patchMap)
}

// ApplyConfigSpecifiedPathsToMedia overlays config media values onto stored settings for the
// paths the config file pins, so a config value always wins over the stored one.
func ApplyConfigSpecifiedPathsToMedia(stored MediaSettings, config Media) (MediaSettings, error) {
	paths := ConfigSpecifiedMediaPaths()
	if len(paths) == 0 {
		return stored, nil
	}
	patchJSON, err := MediaPatchJSONForPaths(config, paths)
	if err != nil {
		return MediaSettings{}, err
	}
	if len(patchJSON) == 0 || string(patchJSON) == "{}" || string(patchJSON) == "null" {
		return stored, nil
	}
	return MergeMediaSettingsPatchJSON(stored, patchJSON)
}

// ValidateMediaPatchNotConfigLocked rejects patches touching config-locked media paths.
func ValidateMediaPatchNotConfigLocked(patchJSON []byte) error {
	locked := ConfigSpecifiedMediaPathSet()
	if len(locked) == 0 {
		return nil
	}
	paths, err := CollectJSONPatchLeafPaths(patchJSON)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if _, ok := locked[path]; ok {
			return fmt.Errorf("%s", MediaFieldConfigLockMessage(path))
		}
	}
	return nil
}
