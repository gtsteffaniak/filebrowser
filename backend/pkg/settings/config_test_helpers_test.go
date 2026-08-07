package settings

import "testing"

func loadConfigForUserDefaultsTests(t *testing.T, configPath string) {
	t.Helper()
	if err := LoadConfigWithDefaultsForTest(configPath); err != nil {
		t.Fatalf("LoadConfigWithDefaultsForTest(%q): %v", configPath, err)
	}
}
