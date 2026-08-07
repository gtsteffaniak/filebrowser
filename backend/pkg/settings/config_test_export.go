package settings

// LoadConfigWithDefaultsForTest loads YAML and runs source wiring used in integration tests.
func LoadConfigWithDefaultsForTest(configPath string) error {
	Config = SetDefaults(true)
	if err := loadConfigWithDefaults(configPath, true); err != nil {
		return err
	}
	if err := ValidateConfig(Config); err != nil {
		return err
	}
	setupSources(true)
	InitializeUserResolvers()
	applyInitialSourceAccessFromLoadedConfig()
	return nil
}

func applyInitialSourceAccessFromLoadedConfig() {
	for _, src := range Config.Server.Sources {
		if src != nil && !src.Config.DefaultPermissions.IsUnset() {
			ApplySourceAccessDefaultsToAllSources(src.Config.DefaultPermissions)
			return
		}
	}
}
