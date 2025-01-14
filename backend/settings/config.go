package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/version"
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
	if len(Config.Server.Sources) > 0 {
		// TODO allow multipe sources not named default
		for _, source := range Config.Server.Sources {
			realPath, err := filepath.Abs(source.Path)
			if err != nil {
				log.Fatalf("Error getting source path: %v", err)
			}
			source.Path = realPath
			source.Name = "default"                   // Modify the local copy of the map value
			Config.Server.Sources["default"] = source // Assign the modified value back to the map
		}
	} else {
		realPath, err := filepath.Abs(Config.Server.Root)
		if err != nil {
			log.Fatalf("Error getting source path: %v", err)
		}
		Config.Server.Sources = map[string]Source{
			"default": {
				Name: "default",
				Path: realPath,
			},
		}
	}
	baseurl := strings.Trim(Config.Server.BaseURL, "/")
	if baseurl == "" {
		Config.Server.BaseURL = "/"
	} else {
		Config.Server.BaseURL = "/" + baseurl + "/"
	}
	if !Config.Frontend.DisableDefaultLinks {
		Config.Frontend.ExternalLinks = append(Config.Frontend.ExternalLinks, ExternalLink{
			Text: "FileBrowser Quantum",
			Url:  "https://github.com/gtsteffaniak/filebrowser",
		})
		Config.Frontend.ExternalLinks = append(Config.Frontend.ExternalLinks, ExternalLink{
			Text:  fmt.Sprintf("(%v)", version.Version),
			Title: version.CommitSHA,
			Url:   "https://github.com/gtsteffaniak/filebrowser/releases/",
		})
		Config.Frontend.ExternalLinks = append(Config.Frontend.ExternalLinks, ExternalLink{
			Text: "Help",
			Url:  "https://github.com/gtsteffaniak/filebrowser/wiki",
		})
	}
	fmt.Println(Config.Server.Logging)
	err = logger.SetupLogger(
		Config.Server.Logging.File.Path,
		Config.Server.Logging.File.Level,
		Config.Server.Logging.Stdout.Level,
	)
	if err != nil {
		log.Fatalf("Failed to set up logger: %v", err)
	}
}

func loadConfigFile(configFile string) []byte {
	// Open and read the YAML file
	yamlFile, err := os.Open(configFile)
	if err != nil {
		log.Fatal(err)
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
			Root:               ".",
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
		Frontend: Frontend{
			Name: "FileBrowser Quantum",
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
			GallerySize:     3,
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
