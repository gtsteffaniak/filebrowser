package settings

import (
	"log"
	"os"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/users"
)

var Config Settings

func Initialize(configFile string) {
	yamlData := loadConfigFile(configFile)
	Config = setDefaults()
	err := yaml.Unmarshal(yamlData, &Config)
	if err != nil {
		log.Fatalf("Error unmarshaling YAML data: %v", err)
	}
	Config.UserDefaults.Perm = Config.UserDefaults.Permissions
	Config.Server.Root = strings.TrimPrefix(Config.Server.Root, "/")
}

func loadConfigFile(configFile string) []byte {
	// Open and read the YAML file
	yamlFile, err := os.Open(configFile)
	if err != nil {
		log.Printf("ERROR: opening config file\n %v\n WARNING: Using default config only\n If this was a mistake, please make sure the file exists and is accessible by the filebrowser binary.\n\n", err)
		Config = setDefaults()
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
		Server: Server{
			EnableThumbnails:   true,
			ResizePreview:      false,
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
			AdminUsername: "admin",
			AdminPassword: "admin",
			Method:        "password",
			Signup:        false,
			Recaptcha: Recaptcha{
				Host: "",
			},
		},
		UserDefaults: UserDefaults{
			Scope:           ".",
			LockPassword:    false,
			HideDotfiles:    true,
			DarkMode:        false,
			DisableSettings: false,
			Locale:          "en",
			Permissions: users.Permissions{
				Create:   false,
				Rename:   false,
				Modify:   false,
				Delete:   false,
				Share:    false,
				Download: false,
				Admin:    false,
			},
		},
	}
}

// Apply applies the default options to a user.
func (d *UserDefaults) Apply(u *users.User) {
	u.DisableSettings = d.DisableSettings
	u.DarkMode = d.DarkMode
	u.Scope = d.Scope
	u.Locale = d.Locale
	u.ViewMode = d.ViewMode
	u.SingleClick = d.SingleClick
	u.Perm = d.Perm
	u.Sorting = d.Sorting
	u.Commands = d.Commands
	u.HideDotfiles = d.HideDotfiles
	u.DateFormat = d.DateFormat
}
