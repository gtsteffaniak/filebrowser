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
	yamlData, err := loadConfigFile(configFile)
	if err != nil && configFile != "config.yaml" {
		logger.Fatal("Could not load specified config file: " + err.Error())
	}
	if err != nil {
		logger.Warning(fmt.Sprintf("Could not load config file '%v', using default settings: %v", configFile, err))
	}
	Config = setDefaults()
	err = yaml.Unmarshal(yamlData, &Config)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Error unmarshaling YAML data: %v", err))
	}
	Config.UserDefaults.Perm = Config.UserDefaults.Permissions
	// Convert relative path to absolute path
	if len(Config.Server.Sources) > 0 {
		// TODO allow multipe sources not named default
		for _, source := range Config.Server.Sources {
			realPath, err2 := filepath.Abs(source.Path)
			if err2 != nil {
				logger.Fatal(fmt.Sprintf("Error getting source path: %v", err2))
			}
			source.Path = realPath
			source.Name = "default"
			Config.Server.Sources = []Source{source} // temporary set only one source
		}
	} else {
		realPath, err2 := filepath.Abs(Config.Server.Root)
		if err2 != nil {
			logger.Fatal(fmt.Sprintf("Error getting source path: %v", err2))
		}
		Config.Server.Sources = []Source{
			{
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
			Text:  fmt.Sprintf("(%v)", version.Version),
			Title: version.CommitSHA,
			Url:   "https://github.com/gtsteffaniak/filebrowser/releases/",
		})
		Config.Frontend.ExternalLinks = append(Config.Frontend.ExternalLinks, ExternalLink{
			Text: "Help",
			Url:  "https://github.com/gtsteffaniak/filebrowser/wiki",
		})
	}
	if len(Config.Server.Logging) == 0 {
		Config.Server.Logging = []LogConfig{
			{
				Output: "stdout",
			},
		}
	}
	for _, logConfig := range Config.Server.Logging {
		err = logger.SetupLogger(
			logConfig.Output,
			logConfig.Levels,
			logConfig.ApiLevels,
			logConfig.NoColors,
		)
		if err != nil {
			log.Println("[ERROR] Failed to set up logger:", err)
		}
	}
}

func loadConfigFile(configFile string) ([]byte, error) {
	// Open and read the YAML file
	yamlFile, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer yamlFile.Close()

	stat, err := yamlFile.Stat()
	if err != nil {
		return nil, err
	}

	yamlData := make([]byte, stat.Size())
	_, err = yamlFile.Read(yamlData)
	if err != nil {
		return nil, err
	}
	return yamlData, nil
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
