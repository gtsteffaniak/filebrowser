package settings

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

const SourceDefaultsConfigLockMessage = "some source access defaults were set in the config file and cannot be changed here until an admin removes them from the config file"

// BuiltinDefaultSourceFilePermissions is used when no source access defaults are configured.
func BuiltinDefaultSourceFilePermissions() users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View:     true,
		Download: true,
		Modify:   false,
		Create:   false,
		Delete:   false,
	}
}

// NormalizeSourceFilePermissions returns built-in defaults when all flags are unset.
func NormalizeSourceFilePermissions(p users.SourceFilePermissions) users.SourceFilePermissions {
	if p.IsUnset() {
		return BuiltinDefaultSourceFilePermissions()
	}
	return p
}

// ApplySourceAccessDefaultsToAllSources copies the same template onto every configured source.
func ApplySourceAccessDefaultsToAllSources(perms users.SourceFilePermissions) {
	p := users.MarkSourceFilePermissionsConfigured(NormalizeSourceFilePermissions(perms))
	for _, src := range Config.Server.Sources {
		if src == nil {
			continue
		}
		src.Config.DefaultPermissions = p
	}
}

// DefaultSourceFilePermissions returns the effective global source access defaults.
func DefaultSourceFilePermissions() users.SourceFilePermissions {
	for _, src := range Config.Server.Sources {
		if src == nil {
			continue
		}
		if !src.Config.DefaultPermissions.IsUnset() {
			return NormalizeSourceFilePermissions(src.Config.DefaultPermissions)
		}
	}
	return BuiltinDefaultSourceFilePermissions()
}

// ConfigSourceDefaultLockedPaths returns API lock paths for config-specified defaultPermissions flags.
func ConfigSourceDefaultLockedPaths() []string {
	if len(Env.ConfigSourceDefaultPermissions) == 0 {
		return nil
	}
	paths := make([]string, 0, len(Env.ConfigSourceDefaultPermissions))
	for flag := range Env.ConfigSourceDefaultPermissions {
		paths = append(paths, "defaultPermissions."+flag)
	}
	sort.Strings(paths)
	return paths
}

// OverlayConfigSourceDefaults applies config-locked permission flags onto stored defaults.
func OverlayConfigSourceDefaults(stored users.SourceFilePermissions) users.SourceFilePermissions {
	if len(Env.ConfigSourceDefaultPermissions) == 0 {
		return stored
	}
	stored = NormalizeSourceFilePermissions(stored)
	for flag, val := range Env.ConfigSourceDefaultPermissions {
		setSourcePermissionFlag(&stored, flag, val)
	}
	return users.MarkSourceFilePermissionsConfigured(stored)
}

func sourcePermissionsPatchWrapper(p users.SourceFilePermissions) map[string]interface{} {
	return map[string]interface{}{
		"defaultPermissions": map[string]interface{}{
			"view":     p.View,
			"download": p.Download,
			"modify":   p.Modify,
			"create":   p.Create,
			"delete":   p.Delete,
		},
	}
}

// MergeSourceDefaultsPatchJSON merges a partial defaultPermissions patch into base permissions.
func MergeSourceDefaultsPatchJSON(base users.SourceFilePermissions, patchJSON []byte) (users.SourceFilePermissions, error) {
	baseBytes, err := json.Marshal(sourcePermissionsPatchWrapper(base))
	if err != nil {
		return users.SourceFilePermissions{}, fmt.Errorf("marshal base source defaults: %w", err)
	}
	mergedBytes, err := mergeUserDefaultsJSON(baseBytes, patchJSON)
	if err != nil {
		return users.SourceFilePermissions{}, err
	}
	var wrapper struct {
		DefaultPermissions users.SourceFilePermissions `json:"defaultPermissions"`
	}
	if err := json.Unmarshal(mergedBytes, &wrapper); err != nil {
		return users.SourceFilePermissions{}, fmt.Errorf("unmarshal merged source defaults: %w", err)
	}
	return NormalizeSourceFilePermissions(wrapper.DefaultPermissions), nil
}

// ValidateSourceDefaultsPatchNotConfigLocked rejects patches touching config-locked permission flags.
func ValidateSourceDefaultsPatchNotConfigLocked(patchJSON []byte) error {
	if len(Env.ConfigSourceDefaultPermissions) == 0 {
		return nil
	}
	paths, err := CollectJSONPatchLeafPaths(patchJSON)
	if err != nil {
		return err
	}
	for _, path := range paths {
		flag := strings.TrimPrefix(path, "defaultPermissions.")
		if _, locked := Env.ConfigSourceDefaultPermissions[flag]; locked {
			return fmt.Errorf("source default %q is locked from config file", path)
		}
	}
	return nil
}
