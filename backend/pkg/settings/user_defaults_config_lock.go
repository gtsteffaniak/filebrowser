package settings

// UserDefaultsConfigLockMessage is returned when userDefaults is present in config.yaml.
const UserDefaultsConfigLockMessage = "user defaults have been set in the config file and cannot be changed until an admin removes the userDefaults from the config file"

// UserDefaultsLockedFromConfig reports whether the loaded config file defined a userDefaults section.
func UserDefaultsLockedFromConfig() bool {
	return Env.ConfigUserDefaultsSpecified
}
