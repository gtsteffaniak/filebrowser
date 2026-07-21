package settings

import "testing"

func TestUserDefaultsLockedFromConfig(t *testing.T) {
	Env.ConfigUserDefaultsSpecified = false
	if UserDefaultsLockedFromConfig() {
		t.Fatal("expected unlocked when config has no userDefaults block")
	}
	Env.ConfigUserDefaultsSpecified = true
	if !UserDefaultsLockedFromConfig() {
		t.Fatal("expected locked when config specifies userDefaults")
	}
	if UserDefaultsConfigLockMessage == "" {
		t.Fatal("expected non-empty lock message")
	}
}
