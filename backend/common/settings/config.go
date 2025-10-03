package settings

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-yaml"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

var Config Settings

const (
	generatorPath = "/relative/or/absolute/path"
)

func Initialize(configFile string) {
	err := loadConfigWithDefaults(configFile, false)
	if err != nil {
		logger.Error("unable to load config, waiting 5 seconds before exiting...")
		time.Sleep(5 * time.Second) // allow sleep time before exiting to give docker/kubernetes time before restarting
		logger.Fatal(err.Error())
	}
	err = ValidateConfig(Config)
	if err != nil {
		errmsg := "The provided config file failed validation. "
		errmsg += "If you are seeing this on a config that worked before, "
		errmsg += "then check the latest releases for breaking changes. "
		errmsg += "visit https://github.com/gtsteffaniak/filebrowser/wiki/Full-Config-Example for more information."
		logger.Error(errmsg)
		time.Sleep(5 * time.Second) // allow sleep time before exiting to give docker/kubernetes time before restarting
		logger.Fatal(err.Error())
	}
	setupFs()
	setupLogging()
	setupAuth(false)
	setupSources(false)
	setupUrls()
	setupFrontend(false)
	setupVideoPreview()
}

func setupFs() {
	// Convert permission values (like 644, 755) to octal interpretation
	filePermOctal, err := strconv.ParseUint(Config.Server.Filesystem.CreateFilePermission, 8, 32)
	if err != nil {
		Config.Server.Filesystem.CreateFilePermission = "644"
		filePermOctal, _ = strconv.ParseUint("644", 8, 32)
	}
	dirPermOctal, err := strconv.ParseUint(Config.Server.Filesystem.CreateDirectoryPermission, 8, 32)
	if err != nil {
		Config.Server.Filesystem.CreateDirectoryPermission = "755"
		dirPermOctal, _ = strconv.ParseUint("755", 8, 32)
	}
	fileutils.SetFsPermissions(os.FileMode(filePermOctal), os.FileMode(dirPermOctal))
}

func setupFrontend(generate bool) {
	if Config.Server.MinSearchLength == 0 {
		Config.Server.MinSearchLength = 3
	}
	if !Config.Frontend.DisableDefaultLinks {
		Config.Frontend.ExternalLinks = append(Config.Frontend.ExternalLinks, ExternalLink{
			Text:  fmt.Sprintf("(%v)", version.Version),
			Title: version.CommitSHA,
			Url:   "https://github.com/gtsteffaniak/filebrowser/releases/",
		})
		Config.Frontend.ExternalLinks = append(Config.Frontend.ExternalLinks, ExternalLink{
			Text: "Help",
			Url:  "help prompt",
		})
	}
	if Config.Frontend.Description == "" {
		Config.Frontend.Description = "FileBrowser Quantum is a file manager for the web which can be used to manage files on your server"
	}
	Config.Frontend.Styling.LightBackground = FallbackColor(Config.Frontend.Styling.LightBackground, "#f5f5f5")
	Config.Frontend.Styling.DarkBackground = FallbackColor(Config.Frontend.Styling.DarkBackground, "#141D24")
	Config.Frontend.Styling.CustomCSSRaw = readCustomCSS(Config.Frontend.Styling.CustomCSS)
	Config.Frontend.Styling.CustomThemeOptions = map[string]CustomTheme{}
	if Config.Frontend.Styling.CustomThemes == nil {
		Config.Frontend.Styling.CustomThemes = map[string]CustomTheme{}
	}
	for name, theme := range Config.Frontend.Styling.CustomThemes {
		addCustomTheme(name, theme.Description, theme.CSS)
	}
	noThemes := len(Config.Frontend.Styling.CustomThemes) == 0
	if noThemes {
		addCustomTheme("default", "The default theme", "")
		// check if file exists
		if _, err := os.Stat("reduce-rounded-corners.css"); err == nil {
			addCustomTheme("alternative", "Reduce rounded corners", "reduce-rounded-corners.css")
			if generate {
				Config.Frontend.Styling.CustomThemes["alternative"] = CustomTheme{
					Description: "Reduce rounded corners",
					CSS:         "reduce-rounded-corners.css",
				}
			}
		}
	}
	_, ok := Config.Frontend.Styling.CustomThemes["default"]
	if !ok {
		addCustomTheme("default", "The default theme", "")
	}

	// Load custom favicon if configured
	loadCustomFavicon()
}

func setupVideoPreview() {
	// If VideoPreview is not initialized, initialize with all types enabled
	if Config.Integrations.Media.Convert.VideoPreview == nil {
		Config.Integrations.Media.Convert.VideoPreview = make(map[VideoPreviewType]bool)
		for _, t := range AllVideoPreviewTypes {
			Config.Integrations.Media.Convert.VideoPreview[t] = true
		}
		return
	}

	// If VideoPreview map is empty, it means user didn't configure any video preview settings
	// In this case, enable all by default
	if len(Config.Integrations.Media.Convert.VideoPreview) == 0 {
		for _, t := range AllVideoPreviewTypes {
			Config.Integrations.Media.Convert.VideoPreview[t] = true
		}
		return
	}

	// User has explicitly configured some video preview settings
	// Start with all enabled, then apply user overrides
	userConfig := make(map[VideoPreviewType]bool)
	for k, v := range Config.Integrations.Media.Convert.VideoPreview {
		userConfig[k] = v
	}

	// Reset to defaults (all enabled)
	Config.Integrations.Media.Convert.VideoPreview = make(map[VideoPreviewType]bool)
	for _, t := range AllVideoPreviewTypes {
		Config.Integrations.Media.Convert.VideoPreview[t] = true
	}

	// Apply user overrides (only for explicitly set values)
	for k, v := range userConfig {
		Config.Integrations.Media.Convert.VideoPreview[k] = v
	}
}

func getRealPath(path string) string {
	realPath, err := filepath.Abs(path)
	if err != nil {
		logger.Fatalf("could not find configured source path: %v", err)
	}
	// check path exists
	if _, err = os.Stat(realPath); os.IsNotExist(err) {
		logger.Fatalf("configured source path does not exist: %v", realPath)
	}
	return realPath
}

func setupSources(generate bool) {
	if len(Config.Server.Sources) == 0 {
		logger.Fatal("There are no `server.sources` configured. If you have `server.root` configured, please update the config and add at least one `server.sources` with a `path` configured.")
	} else {
		for k, source := range Config.Server.Sources {
			if source.Config.Disabled {
				continue
			}
			realPath := getRealPath(source.Path)
			name := filepath.Base(realPath)
			if name == "\\" {
				name = strings.Split(realPath, ":")[0]
			}
			if generate {
				source.Path = generatorPath // use placeholder path
			} else {
				source.Path = realPath // use absolute path
			}
			if source.Name == "" {
				_, ok := Config.Server.SourceMap[source.Path]
				if ok {
					source.Name = name + fmt.Sprintf("-%v", k)
				} else {
					source.Name = name
				}
			}
			modifyExcludeInclude(source)
			if source.Config.DefaultUserScope == "" {
				source.Config.DefaultUserScope = "/"
			}
			Config.Server.SourceMap[source.Path] = source
			Config.Server.NameToSource[source.Name] = source
		}
	}
	// clean up the in memory source list to be accurate and unique
	sourceList := []*Source{}
	defaultScopes := []users.SourceScope{}
	allSourceNames := []string{}
	if len(Config.Server.Sources) == 1 {
		Config.Server.Sources[0].Config.DefaultEnabled = true
	}
	for _, sourcePathOnly := range Config.Server.Sources {
		var realPath string
		if generate {
			// When generating, skip path validation and use the already-set path
			realPath = sourcePathOnly.Path
		} else {
			realPath = getRealPath(sourcePathOnly.Path)
		}
		source, ok := Config.Server.SourceMap[realPath]
		if ok && !slices.Contains(allSourceNames, source.Name) {
			sourceList = append(sourceList, source)
			if source.Config.DefaultEnabled {
				defaultScopes = append(defaultScopes, users.SourceScope{
					Name:  source.Path,
					Scope: source.Config.DefaultUserScope,
				})
			}
			allSourceNames = append(allSourceNames, source.Name)
		} else {
			logger.Warningf("skipping source: %v", sourcePathOnly.Path)
		}
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
	if Config.Server.BaseURL != "/" {
		if Config.Server.ExternalUrl != "" {
			Config.Server.ExternalUrl = strings.TrimSuffix(Config.Server.ExternalUrl, "/") + "/"
			Config.Server.ExternalUrl = strings.TrimSuffix(Config.Server.ExternalUrl, Config.Server.BaseURL)
		}
		if Config.Server.InternalUrl != "" {
			Config.Server.InternalUrl = strings.TrimSuffix(Config.Server.InternalUrl, "/") + "/"
			Config.Server.InternalUrl = strings.TrimSuffix(Config.Server.InternalUrl, Config.Server.BaseURL)
		}
	}
	Config.Integrations.OnlyOffice.Url = strings.Trim(Config.Integrations.OnlyOffice.Url, "/")
	Config.Integrations.OnlyOffice.InternalUrl = strings.Trim(Config.Integrations.OnlyOffice.InternalUrl, "/")
}

func setupAuth(generate bool) {
	if generate {
		Config.Auth.AdminPassword = "admin"
	}
	if Config.Auth.Methods.PasswordAuth.Enabled {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "password")
	}
	if Config.Auth.Methods.ProxyAuth.Enabled {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "proxy")
	}
	if Config.Auth.Methods.OidcAuth.Enabled {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "oidc")
	}
	if Config.Auth.Methods.NoAuth {
		logger.Warning("Configured with no authentication, this is not recommended.")
		Config.Auth.AuthMethods = []string{"disabled"}
	}
	if Config.Auth.Methods.OidcAuth.Enabled || generate {
		err := validateOidcAuth()
		if err != nil && !generate {
			logger.Fatalf("Error validating OIDC auth: %v", err)
		}
		logger.Info("OIDC Auth configured successfully")
	}

	// use password auth as default if no auth methods are set
	if len(Config.Auth.AuthMethods) == 0 {
		Config.Auth.Methods.PasswordAuth.Enabled = true
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "password")
	}
	Config.UserDefaults.LoginMethod = Config.Auth.AuthMethods[0]

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
		// Enable debug logging automatically in dev mode
		levels := logConfig.Levels
		if os.Getenv("FILEBROWSER_DEVMODE") == "true" {
			levels = "info|warning|error|debug"
		}

		logConfig := logger.JsonConfig{
			Levels:     levels,
			ApiLevels:  logConfig.ApiLevels,
			Output:     logConfig.Output,
			Utc:        logConfig.Utc,
			NoColors:   logConfig.NoColors,
			Json:       logConfig.Json,
			Structured: false,
		}
		err := logger.EnableCompatibilityMode(logConfig)
		if err != nil {
			log.Println("[ERROR] Failed to set up logger:", err)
		}
	}
}

func loadConfigWithDefaults(configFile string, isGenerate bool) error {
	Config = setDefaults(isGenerate)
	// Open and read the YAML file
	yamlFile, err := os.Open(configFile)
	if err != nil {
		if configFile != "" {
			logger.Errorf("could not open config file '%v', using default settings.", configFile)
		}
		Config.Server.Sources = []*Source{
			{
				Path: ".",
				Config: SourceConfig{
					DefaultEnabled: true,
				},
			},
		}
		loadEnvConfig()
		return nil
	}
	defer yamlFile.Close()

	// Get the directory containing the config file for reference resolution
	configDir := filepath.Dir(configFile)
	if configDir == "" {
		configDir = "."
	}

	// Create decoder with ReferenceDirs to support multi-config files
	// This enables YAML anchors and references to work across multiple files
	decoder := yaml.NewDecoder(yamlFile,
		yaml.DisallowUnknownField(),
		yaml.ReferenceDirs(configDir),
	)

	err = decoder.Decode(&Config)
	if err != nil {
		// Fallback to old method if multi-config parsing fails
		logger.Debugf("Multi-config parsing failed, falling back to single file parsing: %v", err)

		// Reset file position and try old method
		if _, seekErr := yamlFile.Seek(0, 0); seekErr != nil {
			return fmt.Errorf("failed to read yaml file: %v", seekErr)
		}
		stat, statErr := yamlFile.Stat()
		if statErr != nil {
			return statErr
		}
		yamlData := make([]byte, stat.Size())
		_, readErr := yamlFile.Read(yamlData)
		if readErr != nil && configFile != "config.yaml" {
			return fmt.Errorf("could not load specified config file: %v", readErr.Error())
		}
		if readErr != nil {
			logger.Warningf("Could not load config file '%v', using default settings: %v", configFile, readErr)
		}

		// Use simple decoder without ReferenceDirs
		fallbackErr := yaml.NewDecoder(strings.NewReader(string(yamlData)), yaml.DisallowUnknownField()).Decode(&Config)
		if fallbackErr != nil {
			return fmt.Errorf("error unmarshaling YAML data: %v", fallbackErr)
		}
	}

	loadEnvConfig()
	return nil
}

func ValidateConfig(config Settings) error {
	validate := validator.New()

	// Register custom validator for file permissions
	err := validate.RegisterValidation("file_permission", validateFilePermission)
	if err != nil {
		return fmt.Errorf("could not register file_permission validator: %v", err)
	}

	err = validate.Struct(Config)
	if err != nil {
		return fmt.Errorf("could not validate config: %v", err)
	}
	return nil
}

// validateFilePermission validates that a string is a valid Unix octal file permission (3-4 digits, 0-7)
func validateFilePermission(fl validator.FieldLevel) bool {
	value := fl.Field().String()

	// Must be 3 or 4 characters long
	if len(value) < 3 || len(value) > 4 {
		return false
	}

	// All characters must be octal digits (0-7)
	for _, char := range value {
		if char < '0' || char > '7' {
			return false
		}
	}

	return true
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

	oidcClientId := os.Getenv("FILEBROWSER_OIDC_CLIENT_ID")
	if oidcClientId != "" {
		Config.Auth.Methods.OidcAuth.ClientID = oidcClientId
		logger.Info("Using OIDC Client ID from FILEBROWSER_OIDC_CLIENT_ID environment variable")
	}

	oidcClientSecret := os.Getenv("FILEBROWSER_OIDC_CLIENT_SECRET")
	if oidcClientSecret != "" {
		Config.Auth.Methods.OidcAuth.ClientSecret = oidcClientSecret
		logger.Info("Using OIDC Client Secret from FILEBROWSER_OIDC_CLIENT_SECRET environment variable")
	}

	jwtTokenSecret := os.Getenv("FILEBROWSER_JWT_TOKEN_SECRET")
	if jwtTokenSecret != "" {
		Config.Auth.Key = jwtTokenSecret
		logger.Info("Using JWT Token Secret from FILEBROWSER_JWT_TOKEN_SECRET environment variable")
	}

	totpSecret := os.Getenv("FILEBROWSER_TOTP_SECRET")
	if totpSecret != "" {
		Config.Auth.TotpSecret = totpSecret
		logger.Info("Using TOTP Secret from FILEBROWSER_TOTP_SECRET environment variable")
	}

	recaptchaSecret := os.Getenv("FILEBROWSER_RECAPTCHA_SECRET")
	if recaptchaSecret != "" {
		Config.Auth.Methods.PasswordAuth.Recaptcha.Secret = recaptchaSecret
		logger.Info("Using ReCaptcha Secret from FILEBROWSER_RECAPTCHA_SECRET environment variable")
	}

}

func setDefaults(generate bool) Settings {
	// get number of CPUs available
	numCpus := 4 // default to 4 CPUs if runtime.NumCPU() fails or is not available
	cpus := runtime.NumCPU()
	if cpus > 0 && !generate {
		numCpus = cpus
	}
	database := os.Getenv("FILEBROWSER_DATABASE")
	if database == "" {
		database = "database.db"
	}
	s := Settings{
		Server: Server{
			Port:               80,
			NumImageProcessors: numCpus,
			BaseURL:            "",
			Database:           database,
			SourceMap:          map[string]*Source{},
			NameToSource:       map[string]*Source{},
			MaxArchiveSizeGB:   50,
			CacheDir:           "tmp",
			Filesystem: Filesystem{
				CreateFilePermission:      "644",
				CreateDirectoryPermission: "755",
			},
		},
		Auth: Auth{
			AdminUsername:        "admin",
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
			DisableOnlyOfficeExt: ".md .txt .pdf",
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
			FileLoading: users.FileLoading{
				MaxConcurrent: 10,
				ChunkSize:     10, // 10MB
			},
		},
	}
	// Initialize ImagePreview map with all supported types set to false by default
	s.Integrations.Media.Convert.ImagePreview = make(map[ImagePreviewType]bool)
	for _, t := range AllImagePreviewTypes {
		s.Integrations.Media.Convert.ImagePreview[t] = false
	}

	// Initialize VideoPreview map with all supported types set to true by default
	s.Integrations.Media.Convert.VideoPreview = make(map[VideoPreviewType]bool)
	for _, t := range AllVideoPreviewTypes {
		s.Integrations.Media.Convert.VideoPreview[t] = true
	}
	return s
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
		logger.Debug("Could not get scope from source name: ", sourceName)
		return "", fmt.Errorf("source with name not found %v", sourceName)
	}
	for _, scope := range scopes {
		if scope.Name == source.Path {
			return scope.Scope, nil
		}
	}
	logger.Debugf("scope not found for source %v", sourceName)
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

func loadCustomFavicon() {
	// Check if a custom favicon path is configured
	if Config.Frontend.Favicon == "" {
		logger.Debug("No custom favicon configured, using default")
		return
	}

	// Get absolute path for the favicon
	faviconPath, err := filepath.Abs(Config.Frontend.Favicon)
	if err != nil {
		logger.Warningf("Could not resolve favicon path '%v': %v", Config.Frontend.Favicon, err)
		Config.Frontend.Favicon = "" // Unset invalid path
		return
	}

	// Check if the favicon file exists and get info
	stat, err := os.Stat(faviconPath)
	if err != nil {
		logger.Warningf("Could not access custom favicon file '%v': %v", faviconPath, err)
		Config.Frontend.Favicon = "" // Unset invalid path
		return
	}

	// Check file size (limit to 1MB for security)
	const maxFaviconSize = 1024 * 1024 // 1MB
	if stat.Size() > maxFaviconSize {
		logger.Warningf("Favicon file '%v' is too large (%d bytes), maximum allowed is %d bytes", faviconPath, stat.Size(), maxFaviconSize)
		Config.Frontend.Favicon = "" // Unset invalid path
		return
	}

	// Validate file format based on extension
	ext := strings.ToLower(filepath.Ext(faviconPath))
	switch ext {
	case ".ico", ".png", ".svg":
		// Valid favicon formats
	default:
		logger.Warningf("Unsupported favicon format '%v', supported formats: .ico, .png, .svg", ext)
		Config.Frontend.Favicon = "" // Unset invalid path
		return
	}

	// Update to absolute path and mark as valid
	Config.Frontend.Favicon = faviconPath

	logger.Infof("Successfully validated custom favicon at '%v' (%d bytes, %s)", faviconPath, stat.Size(), ext)
}

func modifyExcludeInclude(config *Source) {
	normalize := func(s []string, checkExists bool) {
		for i, v := range s {
			s[i] = "/" + strings.Trim(v, "/")
			// check if file/folder exists
			if checkExists {
				realPath, err := filepath.Abs(config.Path + s[i])
				if err != nil {
					logger.Warningf("could not get absolute path for %v: %v", s[i], err)
					continue
				}
				if _, err := os.Stat(realPath); os.IsNotExist(err) {
					logger.Warningf("configured exclude/include path %v does not exist", realPath)
				}
			}
		}
	}

	normalize(config.Config.Exclude.FolderPaths, true)
	normalize(config.Config.Exclude.FilePaths, true)
	normalize(config.Config.Include.RootFolders, true)
	normalize(config.Config.Include.RootFiles, true)
	normalize(config.Config.NeverWatchPaths, true)
}
