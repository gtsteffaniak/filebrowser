package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/version"
)

var Config Settings

func Initialize(configFile string) {
	err := loadConfigWithDefaults(configFile)
	if err != nil {
		logger.Fatal(err.Error())
	}
	setupLogging()
	setupAuth()
	setupSources()
	setupBaseURL()
	setupFrontend()
}

func setupFrontend() {
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
}

func setupSources() {

	// Convert relative path to absolute path
	if len(Config.Server.Sources) > 0 {
		if Config.Server.Root != "" {
			logger.Warning("`server.root` is configured but will be ignored in favor of `server.sources`")
		}
		// TODO allow multiple sources not named default
		for _, source := range Config.Server.Sources {
			realPath, err2 := filepath.Abs(source.Path)
			if err2 != nil {
				logger.Fatal(fmt.Sprintf("Error getting source path: %v", err2))
			}
			source.Path = realPath
			source.Name = "default"
			Config.Server.Sources = []Source{source} // temporary set only one source
			Config.Server.DefaultSource = realPath
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
		Config.Server.DefaultSource = realPath
	}
	for _, v := range Config.Server.Sources {
		Config.Server.SourceList = append(Config.Server.SourceList, v.Name+": "+v.Path)
	}
}

func setupBaseURL() {
	baseurl := strings.Trim(Config.Server.BaseURL, "/")
	if baseurl == "" {
		Config.Server.BaseURL = "/"
	} else {
		Config.Server.BaseURL = "/" + baseurl + "/"
	}
}
func setupAuth() {
	if Config.Auth.Method != "" {
		logger.Warning("The `auth.method` setting is deprecated and will be removed in a future version. Please use `auth.methods` instead.")
	}
	Config.UserDefaults.Perm = Config.UserDefaults.Permissions
	if Config.Auth.Methods.PasswordAuth.Enabled {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "Password")
	}
	if Config.Auth.Methods.ProxyAuth.Enabled {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "Proxy")
	}
	if Config.Auth.Methods.NoAuth {
		logger.Warning("Configured with no authentication, this is not recommended.")
		Config.Auth.AuthMethods = []string{"Disabled"}
	}
	// use password auth as default if no auth methods are set
	if len(Config.Auth.AuthMethods) == 0 {
		Config.Auth.Methods.PasswordAuth.Enabled = true
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "Password")
	}
}

func setupLogging() {
	if len(Config.Server.Logging) == 0 {
		Config.Server.Logging = []LogConfig{
			{
				Output: "stdout",
			},
		}
	}
	for _, logConfig := range Config.Server.Logging {
		err := logger.SetupLogger(
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

func loadConfigWithDefaults(configFile string) error {
	// Open and read the YAML file
	yamlFile, err := os.Open(configFile)
	if err != nil {
		return err
	}
	defer yamlFile.Close()

	stat, err := yamlFile.Stat()
	if err != nil {
		return err
	}

	yamlData := make([]byte, stat.Size())
	_, err = yamlFile.Read(yamlData)
	if err != nil && configFile != "config.yaml" {
		return fmt.Errorf("could not load specified config file: " + err.Error())
	}
	if err != nil {
		logger.Warning(fmt.Sprintf("Could not load config file '%v', using default settings: %v", configFile, err))
	}
	Config = setDefaults()
	err = yaml.Unmarshal(yamlData, &Config)
	if err != nil {
		return fmt.Errorf("error unmarshaling YAML data: %v", err)
	}
	return nil
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
			AdminUsername:        "admin",
			AdminPassword:        "admin",
			TokenExpirationHours: 2,
			Signup:               false,
			Recaptcha: Recaptcha{
				Host: "",
			},
			Methods: LoginMethods{
				PasswordAuth: PasswordAuthConfig{
					MinLength: 5,
				},
			},
		},
		Frontend: Frontend{
			Name: "FileBrowser Quantum",
		},
		UserDefaults: UserDefaults{
			DisableOnlyOfficeExt: ".txt .csv .html",
			StickySidebar:        true,
			Scopes: map[string]string{
				"default": "/",
			},
			LockPassword:    false,
			ShowHidden:      false,
			DarkMode:        true,
			DisableSettings: false,
			ViewMode:        "normal",
			Locale:          "en",
			GallerySize:     3,
			Permissions: users.Permissions{
				Modify: false,
				Share:  false,
				Admin:  false,
				Api:    false,
			},
		},
	}
}
