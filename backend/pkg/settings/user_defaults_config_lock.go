package settings

import "fmt"

// UserDefaultsConfigLockMessage explains partial config locking in the admin UI.
const UserDefaultsConfigLockMessage = "some user defaults were set in the config file and cannot be changed here until an admin removes them from the config file"

// UserDefaultsFieldConfigLockMessage formats a single-path lock error.
func UserDefaultsFieldConfigLockMessage(path string) string {
	return fmt.Sprintf("user default %q is locked from config file", path)
}

// UserDefaultsLockedFromConfig reports whether any userDefaults paths were set in config.
func UserDefaultsLockedFromConfig() bool {
	return len(Env.ConfigUserDefaultsSpecifiedPaths) > 0
}
