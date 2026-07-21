package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func runConfigMigrate(args []string) error {
	fs := flag.NewFlagSet("config migrate", flag.ExitOnError)
	configFile := fs.String("c", "config.yaml", "Path to the config file")
	outputFile := fs.String("o", "", "Write migrated config to this path (default: print to stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *configFile == "" {
		return fmt.Errorf("config path is required")
	}

	rawBytes, err := os.ReadFile(*configFile)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	var rawConfig map[string]interface{}
	if err = yaml.Unmarshal(rawBytes, &rawConfig); err != nil {
		return fmt.Errorf("parse config YAML: %w", err)
	}

	udRaw, ok := rawConfig["userDefaults"]
	if !ok {
		fmt.Println("userDefaults section not found; nothing to migrate")
		return nil
	}
	udMap, ok := udRaw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("userDefaults must be a mapping")
	}
	if !settings.NeedsUserDefaultsConfigMigration(udMap) {
		fmt.Println("userDefaults already uses nested v2 structure; nothing to migrate")
		return nil
	}

	migrated, err := settings.MigrateUserDefaultsConfig(udMap)
	if err != nil {
		return err
	}
	rawConfig["userDefaults"] = migrated.UserDefaults

	out, err := yaml.Marshal(rawConfig)
	if err != nil {
		return fmt.Errorf("marshal migrated config: %w", err)
	}

	for _, warning := range migrated.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}

	if *outputFile == "" {
		fmt.Print(string(out))
		return nil
	}
	if err := os.WriteFile(*outputFile, out, 0o600); err != nil {
		return fmt.Errorf("write migrated config: %w", err)
	}
	fmt.Printf("wrote migrated config to %s\n", *outputFile)
	return nil
}

func configMigrateUsage() {
	fmt.Print(`usage: ./filebrowser config migrate [-c config.yaml] [-o migrated.yaml]

Converts deprecated flat userDefaults keys to the nested v2 structure.
Prints warnings for legacy per-source permission keys that belong under
server.sources[].config.defaultPermissions.

`)
}

func runConfigCommand(args []string) error {
	if len(args) == 0 {
		configMigrateUsage()
		return fmt.Errorf("missing config subcommand")
	}
	switch args[0] {
	case "migrate":
		return runConfigMigrate(args[1:])
	case "-h", "help":
		configMigrateUsage()
		return nil
	default:
		configMigrateUsage()
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}
