package settings

import "testing"

func TestUserDefaultsLockedFromConfig(t *testing.T) {
	Env.ConfigUserDefaultsSpecified = false
	Env.ConfigUserDefaultsSpecifiedPaths = nil
	if UserDefaultsLockedFromConfig() {
		t.Fatal("expected unlocked when config has no userDefaults paths")
	}
	Env.ConfigUserDefaultsSpecified = true
	Env.ConfigUserDefaultsSpecifiedPaths = []string{"listing.showHidden"}
	if !UserDefaultsLockedFromConfig() {
		t.Fatal("expected locked when config specifies userDefaults paths")
	}
	if UserDefaultsConfigLockMessage == "" {
		t.Fatal("expected non-empty lock message")
	}
}
