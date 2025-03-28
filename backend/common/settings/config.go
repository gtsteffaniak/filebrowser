package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
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
		logger.Fatal("There are no `server.sources` configured. If you have `server.root` configured, please update the config and add at least one `server.sources` with a `path` configured.")
	} else {
		for k, source := range Config.Server.Sources {
			realPath := getRealPath(source.Path)
			name := filepath.Base(realPath)
			if name == "\\" {
				name = strings.Split(realPath, ":")[0]
			}
			source.Path = realPath // use absolute path
			if source.Name == "" {
				_, ok := Config.Server.SourceMap[source.Path]
				if ok {
					source.Name = name + fmt.Sprintf("-%v", k)
				} else {
					source.Name = name
				}
				if Config.Server.DefaultSource.Path == "" {
					Config.Server.DefaultSource = source
				}
			}
			Config.Server.SourceMap[source.Path] = source
			Config.Server.NameToSource[source.Name] = source
		}
	}
	// clean up the in memory source list to be accurate and unique
	sourceList := []Source{}
	defaultScopes := []users.SourceScope{}
	allSourceNames := []string{}
	first := true
	for _, sourcePathOnly := range Config.Server.Sources {
		realPath := getRealPath(sourcePathOnly.Path)
		source, ok := Config.Server.SourceMap[realPath]
		if ok && !slices.Contains(allSourceNames, source.Name) {
			if first {
				source.Config.DefaultEnabled = true
				Config.Server.SourceMap[source.Path] = source
				Config.Server.NameToSource[source.Name] = source
				Config.Server.DefaultSource = source
			}
			first = false
			sourceList = append(sourceList, source)
			if source.Config.DefaultEnabled {
				Config.Server.DefaultSource = source
				defaultScopes = append(defaultScopes, users.SourceScope{
					Name:  source.Path,
					Scope: source.Config.DefaultUserScope,
				})
			}
			allSourceNames = append(allSourceNames, source.Name)
		} else {
			logger.Warning(fmt.Sprintf("source %v is not configured correctly, skipping", sourcePathOnly.Path))
		}
	}
	Config.UserDefaults.DefaultScopes = defaultScopes
	Config.Server.Sources = sourceList
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
		return fmt.Errorf("could not load specified config file: %v", err.Error())
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
			DisableOnlyOfficeExt: ".txt .csv .html .pdf",
			StickySidebar:        true,
			LockPassword:         false,
			ShowHidden:           false,
			DarkMode:             true,
			DisableSettings:      false,
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

func ConvertToBackendScopes(scopes []users.SourceScope) ([]users.SourceScope, error) {
	if len(scopes) == 0 {
		return Config.UserDefaults.DefaultScopes, nil
	}
	newScopes := []users.SourceScope{}
	for _, scope := range scopes {
		if scope.Scope == "" {
			scope.Scope = "/"
		}
		// first check if its already a path name and keep it
		source, ok := Config.Server.SourceMap[scope.Name]
		if ok {

			newScopes = append(newScopes, users.SourceScope{
				Name:  source.Path, // backend name is path
				Scope: scope.Scope,
			})
			continue
		}

		// check if its the name of a source and convert it to a path
		source, ok = Config.Server.NameToSource[scope.Name]
		if !ok {
			return newScopes, fmt.Errorf("invalid scope for source %v", scope.Name)
		}
		newScopes = append(newScopes, users.SourceScope{
			Name:  source.Path, // backend name is path
			Scope: scope.Scope,
		})
	}
	return newScopes, nil
}

func ConvertToFrontendScopes(scopes []users.SourceScope) []users.SourceScope {
	newScopes := make([]users.SourceScope, 0, len(scopes)) // Preserve original order
	for _, scope := range scopes {
		if source, ok := Config.Server.SourceMap[scope.Name]; ok {
			// Replace scope.Name with source.Path while keeping the same Scope value
			newScopes = append(newScopes, users.SourceScope{
				Name:  source.Name,
				Scope: scope.Scope,
			})
		}
	}
	return newScopes
}

func HasSourceByPath(scopes []users.SourceScope, sourcePath string) bool {
	for _, scope := range scopes {
		if scope.Name == sourcePath {
			return true
		}
	}
	return false
}

func GetScopeFromSourceName(scopes []users.SourceScope, sourceName string) (string, error) {
	source, ok := Config.Server.NameToSource[sourceName]
	if !ok {
		logger.Debug(fmt.Sprint("Could not get scope from source name: ", sourceName))
		return "", fmt.Errorf("source with name not found %v", sourceName)
	}
	for _, scope := range scopes {
		if scope.Name == source.Path {
			return scope.Scope, nil
		}
	}
	logger.Debug(fmt.Sprintf("scope not found for source %v", sourceName))
	return "", fmt.Errorf("scope not found for source %v", sourceName)
}

func GetScopeFromSourcePath(scopes []users.SourceScope, sourcePath string) (string, error) {
	for _, scope := range scopes {
		if scope.Name == sourcePath {
			return scope.Scope, nil
		}
	}
	return "", fmt.Errorf("scope not found for source %v", sourcePath)
}

// assumes backend style scopes
func GetSources(u *users.User) []string {
	sources := []string{}
	for _, scope := range u.Scopes {
		source, ok := Config.Server.SourceMap[scope.Name]
		if ok {
			sources = append(sources, source.Name)
		}
	}
	return sources
}
