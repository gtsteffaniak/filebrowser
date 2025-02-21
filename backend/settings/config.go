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
	setupUserScopes()
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

func getRealPath(path string) string {
	realPath, err := filepath.Abs(path)
	if err != nil {
		logger.Fatal(fmt.Sprintf("could not find configured source path: %v", err))
	}
	// check path exists
	if _, err = os.Stat(realPath); os.IsNotExist(err) {
		logger.Fatal(fmt.Sprintf("configured source path does not exist: %v", realPath))
	}
	return realPath
}

func setupSources() {

	if len(Config.Server.Sources) == 0 {
		logger.Warning("`server.root` is deprecated, please update the config to use `server.sources`")
		realPath := getRealPath(Config.Server.Root)
		source := Source{Name: "default", Path: realPath}
		Config.Server.SourceMap[source.Path] = source
		Config.Server.NameToSource["default"] = source
	} else {
		if Config.Server.Root != "" {
			logger.Warning("`server.root` is configured but will be ignored in favor of `server.sources`")
		}
		for k, source := range Config.Server.Sources {
			realPath := getRealPath(source.Path)
			source.Path = realPath // use absolute path
			if source.Name == "" {
				if k == 0 {
					source.Name = "default"
				} else {
					source.Name = "source" + fmt.Sprintf("%v", k)
				}
				Config.Server.DefaultSource = source
			}
			Config.Server.SourceMap[source.Path] = source
			Config.Server.NameToSource[source.Name] = source
		}
	}
	// if only one source listed, make sure its default
	if len(Config.Server.SourceMap) == 1 {
		for _, source := range Config.Server.SourceMap {
			Config.Server.DefaultSource = source
		}
	}
}

func setupUserScopes() {
	for _, source := range Config.Server.SourceMap {
		Config.UserDefaults.Scopes[source.Path] = source.Config.DefaultUserScope
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
			SourceMap:          map[string]Source{},
			NameToSource:       map[string]Source{},
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
			LockPassword:         false,
			ShowHidden:           false,
			DarkMode:             true,
			DisableSettings:      false,
			Scopes:               map[string]string{},
			ViewMode:             "normal",
			Locale:               "en",
			GallerySize:          3,
			Permissions: users.Permissions{
				Modify: false,
				Share:  false,
				Admin:  false,
				Api:    false,
			},
		},
	}
}
