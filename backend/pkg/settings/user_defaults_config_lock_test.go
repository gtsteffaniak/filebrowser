package settings

import "testing"

func TestUserDefaultsLockedFromConfig(t *testing.T) {
	prevSpecified := Env.ConfigUserDefaultsSpecified
	prevPaths := Env.ConfigUserDefaultsSpecifiedPaths
	t.Cleanup(func() {
		Env.ConfigUserDefaultsSpecified = prevSpecified
		Env.ConfigUserDefaultsSpecifiedPaths = prevPaths
	})

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

func TestUserDefaultsFieldConfigLockMessage(t *testing.T) {
	got := UserDefaultsFieldConfigLockMessage("listing.showHidden")
	want := `user default "listing.showHidden" is locked from config file`
	if got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
}
