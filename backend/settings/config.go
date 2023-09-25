package settings

import (
	"log"
	"os"

	"github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/users"
)

var GlobalConfiguration Settings

func Initialize(configFile string) {
	yamlData := loadConfigFile(configFile)
	GlobalConfiguration = setDefaults()
	err := yaml.Unmarshal(yamlData, &GlobalConfiguration)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	GlobalConfiguration.UserDefaults.Perm = GlobalConfiguration.UserDefaults.Permissions
	GlobalConfiguration.Server.Root = "/srv" // hardcoded for now. TODO allow changing
}

func loadConfigFile(configFile string) []byte {
	// Open and read the YAML file
	yamlFile, err := os.Open(configFile)
	if err != nil {
		log.Printf("ERROR: opening config file\n %v\n WARNING: Using default config only\n If this was a mistake, please make sure the file exists and is accessible by the filebrowser binary.\n\n", err)
		GlobalConfiguration = setDefaults()
		return []byte{}
	}
	defer yamlFile.Close()

	stat, err := yamlFile.Stat()
	if err != nil {
		log.Fatalf("Error getting file information: %s", err.Error())
	}

	yamlData := make([]byte, stat.Size())
	_, err = yamlFile.Read(yamlData)
	if err != nil {
		log.Fatalf("Error reading YAML data: %v", err)
	}
	return yamlData
}

func setDefaults() Settings {
	return Settings{
		Signup:        true,
		AdminUsername: "admin",
		AdminPassword: "admin",
		Server: Server{
			EnableThumbnails:   true,
			EnableExec:         false,
			IndexingInterval:   5,
			Port:               8080,
			NumImageProcessors: 4,
			BaseURL:            "",
			Database:           "database.db",
			Log:                "stdout",
			Root:               "/srv",
		},
		Auth: Auth{
			Method: "password",
			Recaptcha: Recaptcha{
				Host: "",
			},
		},
		UserDefaults: UserDefaults{
			Scope:        ".",
			LockPassword: false,
			HideDotfiles: true,
			Permissions: users.Permissions{
				Create:   true,
				Rename:   true,
				Modify:   true,
				Delete:   true,
				Share:    true,
				Download: true,
			},
		},
	}
}
