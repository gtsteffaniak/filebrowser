package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLConfig_sharePolicyIsLoaded(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	yaml := fmt.Sprintf("server:\n  sources:\n    - path: %q\nsharePolicy:\n  maxExpirationHours: 168\n  requirePassword: true\n", filepath.ToSlash(dir))
	if err := os.WriteFile(cfg, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	loadConfigForUserDefaultsTests(t, cfg)
	if Config.SharePolicy.MaxExpirationHours != 168 || !Config.SharePolicy.RequirePassword {
		t.Fatalf("sharePolicy from yaml not loaded: %+v", Config.SharePolicy)
	}
}
