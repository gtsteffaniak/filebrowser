package settings

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/backend/users"
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
	// Convert relative path to absolute path
	realRoot, err := filepath.Abs(Config.Server.Root)
	if err != nil {
		log.Fatalf("Error getting root path: %v", err)
	}
	_, err = os.Stat(realRoot)
	if err != nil {
		log.Fatalf("ERROR: Configured Root Path does not exist! %v", err)
	}
	Config.Server.Root = realRoot
	baseurl := strings.Trim(Config.Server.BaseURL, "/")
	if baseurl == "" {
		Config.Server.BaseURL = "/"
	} else {
		Config.Server.BaseURL = "/" + baseurl + "/"
	}
}

func loadConfigFile(configFile string) []byte {
	// Open and read the YAML file
	yamlFile, err := os.Open(configFile)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}
	defer yamlFile.Close()

	stat, err := yamlFile.Stat()
	if err != nil {
		log.Fatalf("error getting file information: %s", err.Error())
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
			Port:               80,
			NumImageProcessors: 4,
			BaseURL:            "",
			Database:           "database.db",
			Log:                "stdout",
			Root:               "/srv",
			Indexing:           true,
		},
		Auth: Auth{
			TokenExpirationTime: "2h",
			AdminUsername:       "admin",
			AdminPassword:       "admin",
			Method:              "password",
			Signup:              false,
			Recaptcha: Recaptcha{
				Host: "",
			},
		},
		UserDefaults: UserDefaults{
			StickySidebar:   true,
			Scope:           ".",
			LockPassword:    false,
			HideDotfiles:    true,
			DarkMode:        false,
			DisableSettings: false,
			ViewMode:        "normal",
			Locale:          "en",
			Permissions: users.Permissions{
				Create:   false,
				Rename:   false,
				Modify:   false,
				Delete:   false,
				Share:    false,
				Download: false,
				Admin:    false,
				Api:      false,
			},
		},
	}
}
