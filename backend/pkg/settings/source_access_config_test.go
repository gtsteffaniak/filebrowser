package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestOverlayConfigSourceDefaults(t *testing.T) {
	prev := Env.ConfigSourceDefaultPermissions
	t.Cleanup(func() { Env.ConfigSourceDefaultPermissions = prev })

	Env.ConfigSourceDefaultPermissions = map[string]bool{"modify": false}
	stored := users.SourceFilePermissions{View: true, Download: true, Modify: true, Create: true}
	merged := OverlayConfigSourceDefaults(stored)
	if merged.Modify {
		t.Fatal("expected modify overlaid from config")
	}
	if !merged.Create {
		t.Fatal("expected create unchanged")
	}
}

func TestOverlayConfigSourceDefaults_unsetStoredPartialConfig(t *testing.T) {
	prev := Env.ConfigSourceDefaultPermissions
	t.Cleanup(func() { Env.ConfigSourceDefaultPermissions = prev })

	Env.ConfigSourceDefaultPermissions = map[string]bool{"modify": false}
	merged := OverlayConfigSourceDefaults(users.SourceFilePermissions{})
	if !merged.View || !merged.Download {
		t.Fatalf("expected built-in view/download preserved: %+v", merged)
	}
	if merged.Modify {
		t.Fatal("expected modify false from config overlay")
	}
	if !merged.Configured {
		t.Fatal("expected result marked configured")
	}
}

func TestValidateSourceDefaultsPatchNotConfigLocked(t *testing.T) {
	prev := Env.ConfigSourceDefaultPermissions
	t.Cleanup(func() { Env.ConfigSourceDefaultPermissions = prev })

	Env.ConfigSourceDefaultPermissions = map[string]bool{"modify": false}
	if err := ValidateSourceDefaultsPatchNotConfigLocked([]byte(`{"defaultPermissions":{"view":false}}`)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := ValidateSourceDefaultsPatchNotConfigLocked([]byte(`{"defaultPermissions":{"modify":true}}`)); err == nil {
		t.Fatal("expected config lock error")
	}
}

func TestConfigSourceDefaultLockedPaths(t *testing.T) {
	prev := Env.ConfigSourceDefaultPermissions
	t.Cleanup(func() { Env.ConfigSourceDefaultPermissions = prev })

	Env.ConfigSourceDefaultPermissions = map[string]bool{"modify": false, "view": true}
	paths := ConfigSourceDefaultLockedPaths()
	if len(paths) != 2 {
		t.Fatalf("paths=%v", paths)
	}
}

func TestAttachSourceDefaultPermissionsFromConfig(t *testing.T) {
	Config.Server.Sources = []*Source{{Path: "/data", Config: SourceConfig{}}}
	raw := map[string]interface{}{
		"server": map[string]interface{}{
			"sources": []interface{}{
				map[string]interface{}{
					"path": "/data",
					"config": map[string]interface{}{
						"defaultPermissions": map[string]interface{}{
							"modify": false,
						},
					},
				},
			},
		},
	}
	attachSourceDefaultPermissionsFromConfig(raw)
	got := Config.Server.Sources[0].Config.DefaultPermissionsFromConfig
	if got["modify"] != false || len(got) != 1 {
		t.Fatalf("DefaultPermissionsFromConfig: %+v", got)
	}
}
