package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
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
	err = ValidateConfig(Config)
	if err != nil {
		errmsg := "the provided config file failed validation. "
		errmsg += "If you are seeing this on a config that worked previeously, "
		errmsg += "this is because v0.6.8 requires a fully validated config to run. "
		errmsg += "Please review your config for typos and invalid keys which are no longer supported. "
		errmsg += "visit https://github.com/gtsteffaniak/filebrowser/wiki/Full-Config-Example for more information."
		logger.Error(errmsg)
		logger.Fatal(err.Error())
	}
	setupLogging()
	setupAuth()
	setupSources()
	setupUrls()
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
			if source.Config.DefaultUserScope == "" {
				source.Config.DefaultUserScope = "/"
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
	potentialDefaultSource := Config.Server.DefaultSource
	for _, sourcePathOnly := range Config.Server.Sources {
		realPath := getRealPath(sourcePathOnly.Path)
		source, ok := Config.Server.SourceMap[realPath]
		if ok && !slices.Contains(allSourceNames, source.Name) {
			if first {
				potentialDefaultSource = source
			}
			first = false
			sourceList = append(sourceList, source)
			if source.Config.DefaultEnabled {
				if Config.Server.DefaultSource.Path == "" {
					Config.Server.DefaultSource = source
				}
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
	if Config.Server.DefaultSource.Path == "" {
		Config.Server.DefaultSource = potentialDefaultSource
	}
	Config.UserDefaults.DefaultScopes = defaultScopes
	Config.Server.Sources = sourceList
}

func setupUrls() {
	baseurl := strings.Trim(Config.Server.BaseURL, "/")
	if baseurl == "" {
		Config.Server.BaseURL = "/"
	} else {
		Config.Server.BaseURL = "/" + baseurl + "/"
	}
	Config.Server.InternalUrl = strings.Trim(Config.Server.InternalUrl, "/")
	Config.Server.ExternalUrl = strings.Trim(Config.Server.ExternalUrl, "/")
	Config.Integrations.OnlyOffice.Url = strings.Trim(Config.Integrations.OnlyOffice.Url, "/")
	Config.Integrations.OnlyOffice.InternalUrl = strings.Trim(Config.Integrations.OnlyOffice.InternalUrl, "/")
}

func setupAuth() {
	if Config.Auth.Methods.PasswordAuth.Enabled {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "Password")
	}
	if Config.Auth.Methods.ProxyAuth.Enabled {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "Proxy")
	}
	if Config.Auth.Methods.OidcAuth.Enabled {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "OIDC")
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
	Config = setDefaults()
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
	err = yaml.NewDecoder(strings.NewReader(string(yamlData)), yaml.DisallowUnknownField()).Decode(&Config)
	if err != nil {
		return fmt.Errorf("error unmarshaling YAML data: %v", err)
	}
	loadEnvConfig()
	return nil
}

func ValidateConfig(config Settings) error {

	validate := validator.New()
	err := validate.Struct(Config)
	if err != nil {
		return fmt.Errorf("could not validate config: %v", err)
	}
	return nil
}

func loadEnvConfig() {

	adminPassword, ok := os.LookupEnv("FILEBROWSER_ADMIN_PASSWORD")
	if ok {
		logger.Info("Using admin password from FILEBROWSER_ADMIN_PASSWORD environment variable")
		Config.Auth.AdminPassword = adminPassword
	}
	officeSecret, ok := os.LookupEnv("FILEBROWSER_ONLYOFFICE_SECRET")
	if ok {
		logger.Info("Using OnlyOffice secret from FILEBROWSER_ONLYOFFICE_SECRET environment variable")
		Config.Integrations.OnlyOffice.Secret = officeSecret
	}

	ffmpegPath, ok := os.LookupEnv("FILEBROWSER_FFMPEG_PATH")
	if ok {
		Config.Integrations.Media.FfmpegPath = ffmpegPath
	}

}

func setDefaults() Settings {
	return Settings{
		Server: Server{
			Port:               80,
			NumImageProcessors: 4,
			BaseURL:            "",
			Database:           "database.db",
			SourceMap:          map[string]Source{},
			NameToSource:       map[string]Source{},
			MaxArchiveSizeGB:   50,
			CacheDir:           "tmp",
		},
		Auth: Auth{
			AdminUsername:        "admin",
			AdminPassword:        "admin",
			TokenExpirationHours: 2,
			Methods: LoginMethods{
				PasswordAuth: PasswordAuthConfig{
					Enabled:   true,
					MinLength: 5,
					Signup:    false,
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
			ThemeColor:           "var(--blue)",
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

		// first check if its already a path name and keep it
		source, ok := Config.Server.SourceMap[scope.Name]
		if ok {
			if scope.Scope == "" {
				scope.Scope = source.Config.DefaultUserScope
			}
			if !strings.HasPrefix(scope.Scope, "/") {
				scope.Scope = "/" + scope.Scope
			}
			if scope.Scope != "/" && strings.HasSuffix(scope.Scope, "/") {
				scope.Scope = strings.TrimSuffix(scope.Scope, "/")
			}
			newScopes = append(newScopes, users.SourceScope{
				Name:  source.Path, // backend name is path
				Scope: scope.Scope,
			})
			continue
		}

		// check if its the name of a source and convert it to a path
		source, ok = Config.Server.NameToSource[scope.Name]
		if !ok {
			// source might no longer be configured
			continue
		}
		if scope.Scope == "" {
			scope.Scope = source.Config.DefaultUserScope
		}
		if !strings.HasPrefix(scope.Scope, "/") {
			scope.Scope = "/" + scope.Scope
		}
		if scope.Scope != "/" && strings.HasSuffix(scope.Scope, "/") {
			scope.Scope = strings.TrimSuffix(scope.Scope, "/")
		}
		newScopes = append(newScopes, users.SourceScope{
			Name:  source.Path, // backend name is path
			Scope: scope.Scope,
		})
	}
	return newScopes, nil
}

func ConvertToFrontendScopes(scopes []users.SourceScope) []users.SourceScope {
	newScopes := []users.SourceScope{}
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
