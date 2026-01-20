package settings

import (
	"bytes"
	"fmt"
	"io"
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
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
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
	setupLogging()
	// setup logging first to ensure we log any errors
	setupEnv()
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
	setupServer()
	setupAuth(false)
	setupSources(false)
	InitializeUserResolvers() // Initialize user package resolvers after sources are set up
	setupUrls()
	setupFrontend(false)
	setupMedia()
}

func setupServer() {
	if Config.Server.ListenAddress == "" {
		Config.Server.ListenAddress = "0.0.0.0"
	}
	// Check environment variable first (overrides config file)
	if os.Getenv("FILEBROWSER_SQL_WAL") == "true" {
		Config.Server.IndexSqlConfig.WalMode = true
	}
	// WalMode is false by default (OFF journaling)
}

func setupEnv() {
	Env.IsPlaywright = os.Getenv("FILEBROWSER_PLAYWRIGHT_TEST") == "true"
	if Env.IsPlaywright {
		logger.Warning("Running in playwright test mode. This is not recommended for production.")
	}
	Env.IsDevMode = os.Getenv("FILEBROWSER_DEVMODE") == "true"
	if Env.IsDevMode {
		logger.Warning("Running in dev mode. This is not recommended for production.")
	}
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

	// Perform mandatory cache directory speed test
	testCacheDirSpeed()

	logger.Infof("cache directory setup successfully: %v", Config.Server.CacheDir)

}

// testCacheDirSpeed performs a mandatory speed test on the cache directory by writing,
// reading, and deleting a 10MB test file. Reports write and read performance in MB/s.
func testCacheDirSpeed() {
	msgPrfx := "cacheDir"
	failSuffix := "Please review documentation to ensure a valid cache directory is configured https://filebrowserquantum.com/en/docs/configuration/server/#cachedir"
	const testFileSize = 10 * 1024 * 1024 // 10MB

	// Ensure cache directory exists
	err := os.MkdirAll(Config.Server.CacheDir, fileutils.PermDir)
	if err != nil {
		logger.Fatalf("%s failed to create cache directory: %v\n%s", msgPrfx, err, failSuffix)
	}

	testFileName := filepath.Join(Config.Server.CacheDir, "speed_test.tmp")

	// Create test data (10MB of zeros)
	testData := make([]byte, testFileSize)

	// Test write performance
	writeStart := time.Now()
	file, err := os.Create(testFileName)
	if err != nil {
		logger.Fatalf("%s failed to create test file: %v\n%s", msgPrfx, err, failSuffix)
	}

	written, err := file.Write(testData)
	if err != nil {
		file.Close()
		os.Remove(testFileName)
		logger.Fatalf("%s failed to write test file: %v\n%s", msgPrfx, err, failSuffix)
	}

	err = file.Sync() // Ensure data is written to disk
	if err != nil {
		file.Close()
		os.Remove(testFileName)
		logger.Fatalf("%s failed to sync test file to disk: %v\n%s", msgPrfx, err, failSuffix)
	}

	err = file.Close()
	if err != nil {
		os.Remove(testFileName)
		logger.Fatalf("%s failed to close test file: %v\n%s", msgPrfx, err, failSuffix)
	}

	writeDuration := time.Since(writeStart)
	writeSpeedMBs := float64(written) / (1024 * 1024) / writeDuration.Seconds()

	// Test read performance
	readStart := time.Now()
	file, err = os.Open(testFileName)
	if err != nil {
		os.Remove(testFileName)
		logger.Fatalf("%s failed to open test file for reading: %v\n%s", msgPrfx, err, failSuffix)
	}

	readData := make([]byte, testFileSize)
	readBytes, err := io.ReadFull(file, readData)
	if err != nil {
		file.Close()
		os.Remove(testFileName)
		logger.Fatalf("%s failed to read test file: %v\n%s", msgPrfx, err, failSuffix)
	}

	err = file.Close()
	if err != nil {
		os.Remove(testFileName)
		logger.Fatalf("%s failed to close test file after reading: %v\n%s", msgPrfx, err, failSuffix)
	}

	readDuration := time.Since(readStart)
	readSpeedMBs := float64(readBytes) / (1024 * 1024) / readDuration.Seconds()

	// Verify data integrity
	if readBytes != written {
		os.Remove(testFileName)
		logger.Fatalf("%s data integrity check failed: wrote %d bytes but read %d bytes\n%s", msgPrfx, written, readBytes, failSuffix)
	}

	// Clean up test file
	err = os.Remove(testFileName)
	if err != nil {
		logger.Fatalf("%s failed to remove test file: %v\n%s", msgPrfx, err, failSuffix)
	}

	// Log performance results
	writeDurationMs := writeDuration.Seconds() * 1000
	readDurationMs := readDuration.Seconds() * 1000
	const highLatencyThresholdMs = 1000.0 // 1 second for 10MB file indicates high latency

	if writeSpeedMBs < 50 {
		slowSuffix := " Ensure you configure a faster cache directory via the `server.cacheDir` configuration option."
		logger.Warningf("%s slow write speed detected: %.2f MB/s (%.2f ms)\n%s\n%s", msgPrfx, writeSpeedMBs, writeDurationMs, failSuffix, slowSuffix)
	} else if writeDurationMs > highLatencyThresholdMs {
		slowSuffix := " Ensure you configure a faster cache directory via the `server.cacheDir` configuration option."
		logger.Warningf("%s high write latency detected: %.2f ms for 10MB file (speed: %.2f MB/s). This may indicate network storage or I/O issues.\n%s\n%s", msgPrfx, writeDurationMs, writeSpeedMBs, failSuffix, slowSuffix)
	} else {
		logger.Debugf("%s write speed: %.2f MB/s (%.2f ms)", msgPrfx, writeSpeedMBs, writeDurationMs)
	}
	if readSpeedMBs < 50 {
		slowSuffix := " Ensure you configure a faster cache directory via the `server.cacheDir` configuration option."
		logger.Warningf("%s slow read speed detected: %.2f MB/s (%.2f ms)\n%s\n%s", msgPrfx, readSpeedMBs, readDurationMs, failSuffix, slowSuffix)
	} else if readDurationMs > highLatencyThresholdMs {
		slowSuffix := " Ensure you configure a faster cache directory via the `server.cacheDir` configuration option."
		logger.Warningf("%s high read latency detected: %.2f ms for 10MB file (speed: %.2f MB/s). This may indicate network storage or I/O issues.\n%s\n%s", msgPrfx, readDurationMs, readSpeedMBs, failSuffix, slowSuffix)
	} else {
		logger.Debugf("%s read speed : %.2f MB/s (%.2f ms)", msgPrfx, readSpeedMBs, readDurationMs)
	}
	// check cache directory disk free space
	freeSpace, err := fileutils.GetFreeSpace(Config.Server.CacheDir)
	if err != nil {
		logger.Fatalf("%s failed to get free space for cache directory: %v\n%s", msgPrfx, err, failSuffix)
	}
	freeSpaceGB := float64(freeSpace) / (1024 * 1024 * 1024)
	logger.Debugf("%s cache directory has %.2f GB of free space", msgPrfx, freeSpaceGB)
	const minRecommendedGB = 20.0
	if freeSpaceGB < minRecommendedGB {
		logger.Warningf("%s only has %.2f GB of free space, this is less than the %.0f GB minimum recommended free space.\n%s", msgPrfx, freeSpaceGB, minRecommendedGB, failSuffix)
	}
}

func setupFrontend(generate bool) {
	// Load login icon configuration at startup
	loadLoginIcon()
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
	var err error
	if Config.Frontend.Styling.CustomCSS != "" {
		Config.Frontend.Styling.CustomCSSRaw, err = readCustomCSS(Config.Frontend.Styling.CustomCSS)
		if err != nil {
			logger.Warning(err.Error())
		}
	}
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

func setupMedia() {
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

func setupSources(generate bool) {
	if len(Config.Server.Sources) == 0 {
		logger.Fatal("There are no `server.sources` configured. If you have `server.root` configured, please update the config and add at least one `server.sources` with a `path` configured.")
	} else {
		for k, source := range Config.Server.Sources {
			if source.Config.Disabled {
				continue
			}
			realPath, err := filepath.Abs(source.Path)
			if err != nil {
				logger.Fatalf("error getting real path for source %v: %v", source.Path, err)
			}
			exists := utils.CheckPathExists(realPath)
			if !exists {
				logger.Warningf("source path %v is currently not available", realPath)
			}
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
			setConditionals(source)
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
		absPath := sourcePathOnly.Path
		if generate {
			// When generating, skip path validation and use the already-set path
			absPath = sourcePathOnly.Path
		}
		source, ok := Config.Server.SourceMap[absPath]
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

func loadConfigWithDefaults(configFile string, generate bool) error {
	Config = setDefaults(generate)

	// Check if config file exists
	if _, err := os.Stat(configFile); err != nil {
		if configFile != "" {
			logger.Errorf("could not open config file '%v', using default settings.", configFile)
		}
		Config.Server.Sources = []*Source{
			{
				Path: ".",
			},
		}
		loadEnvConfig()
		return nil
	}

	// Try multi-file config first (combine all YAML files in the directory)
	combinedYAML, err := combineYAMLFiles(configFile)
	if err != nil {
		return fmt.Errorf("failed to combine YAML files: %v", err)
	}

	// First pass: Unmarshal into a generic map to resolve all anchors and aliases
	// This allows YAML anchors defined in auxiliary files to be properly merged
	var rawConfig map[string]interface{}
	err = yaml.Unmarshal(combinedYAML, &rawConfig)
	if err != nil {
		return fmt.Errorf("error parsing YAML data: %v", err)
	}

	// Filter to only keep valid top-level Settings struct fields
	// This removes anchor definitions that are just templates (e.g., "test_server: &test_server")
	validFields := map[string]bool{
		"server":       true,
		"auth":         true,
		"integrations": true,
		"frontend":     true,
		"userDefaults": true,
	}

	filteredConfig := make(map[string]interface{})
	for key, value := range rawConfig {
		if validFields[key] {
			filteredConfig[key] = value
		}
	}

	// Marshal the filtered config back to YAML
	filteredYAML, err := yaml.Marshal(filteredConfig)
	if err != nil {
		return fmt.Errorf("error re-marshaling filtered YAML: %v", err)
	}

	// Second pass: Decode with strict validation (disallow unknown fields within valid sections)
	decoder := yaml.NewDecoder(bytes.NewReader(filteredYAML), yaml.DisallowUnknownField())
	err = decoder.Decode(&Config)
	if err != nil {
		return fmt.Errorf("error unmarshaling YAML data: %v", err)
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
	} else {
		// check if database file exists
		if _, err := os.Stat(database); os.IsNotExist(err) {
			database = "database.db"
		}
	}
	if _, err := os.Stat(database); os.IsNotExist(err) {
		logger.Warning("database file could not be found. If this is unexpected, the default path is `./database.db`, but it can be configured in the config file under `server.database`.")
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
			IndexSqlConfig: IndexSqlConfig{
				WalMode:      false,
				BatchSize:    1000,
				CacheSizeMB:  32,
				DisableReuse: false,
			},
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
			DisableOnlyOfficeExt: ".md .txt .pdf .html .xml",
			StickySidebar:        true,
			LockPassword:         false,
			ShowHidden:           false,
			DarkMode:             boolPtr(true),
			DisableSettings:      false,
			ViewMode:             "normal",
			Locale:               "en",
			GallerySize:          3,
			ThemeColor:           "var(--blue)",
			Permissions: UserDefaultsPermissions{
				Modify:   false,
				Share:    false,
				Admin:    false,
				Api:      false,
				Download: boolPtr(true), // defaults to true
			},
			Preview: UserDefaultsPreview{
				HighQuality:        boolPtr(true),
				Image:              boolPtr(true),
				Video:              boolPtr(true),
				MotionVideoPreview: boolPtr(true),
				Office:             boolPtr(true),
				PopUp:              boolPtr(true),
				AutoplayMedia:      boolPtr(true),
				Folder:             boolPtr(true),
			},
			FileLoading: users.FileLoading{
				MaxConcurrent:   10,
				UploadChunkSize: 10, // 10MB
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

// validateCustomImage validates a custom image file path and returns the absolute path or error
func validateCustomImage(configPath, imageName string, maxSize int64, allowedFormats []string) (absolutePath string, err error) {
	// Get absolute path
	absolutePath, err = filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %w", err)
	}

	// Check if file exists
	stat, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("could not access file: %w", err)
	}

	// Check file size
	if stat.Size() > maxSize {
		return "", fmt.Errorf("file too large (%d bytes), maximum allowed is %d bytes", stat.Size(), maxSize)
	}

	// Validate file format
	ext := strings.ToLower(filepath.Ext(absolutePath))
	validFormat := false
	for _, format := range allowedFormats {
		if ext == format {
			validFormat = true
			break
		}
	}
	if !validFormat {
		return "", fmt.Errorf("unsupported format '%s', supported formats: %v", ext, allowedFormats)
	}

	return absolutePath, nil
}

func loadCustomFavicon() {
	const imageName = "favicon"
	const maxSize = 1024 * 1024 // 1MB
	allowedFormats := []string{".ico", ".png", ".svg"}

	// Set default embedded favicon path
	Env.FaviconEmbeddedPath = "img/icons/favicon.svg"

	// Check if a custom favicon path is configured
	if Config.Frontend.Favicon == "" {
		Env.FaviconPath = Env.FaviconEmbeddedPath
		Env.FaviconIsCustom = false
		return
	}

	// Validate custom favicon
	validatedPath, err := validateCustomImage(Config.Frontend.Favicon, imageName, maxSize, allowedFormats)
	if err != nil {
		logger.Warningf("Custom favicon validation failed: %v, using default", err)
		Config.Frontend.Favicon = ""
		Env.FaviconPath = Env.FaviconEmbeddedPath
		Env.FaviconIsCustom = false
		return
	}

	// Update to validated path and mark as custom
	Config.Frontend.Favicon = validatedPath
	Env.FaviconPath = validatedPath
	Env.FaviconIsCustom = true
	logger.Infof("Using custom favicon: %s", Env.FaviconPath)
}

func loadLoginIcon() {
	const imageName = "login icon"
	const maxSize = 1024 * 1024 // 1MB
	allowedFormats := []string{".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico"}

	// Set default embedded icon path based on dark mode preference
	isDarkMode := Config.UserDefaults.DarkMode != nil && *Config.UserDefaults.DarkMode
	if isDarkMode {
		Env.LoginIconEmbeddedPath = "img/icons/favicon.svg" // Dark mode: dark background
	} else {
		Env.LoginIconEmbeddedPath = "img/icons/favicon-light.svg" // Light mode: light background
	}

	// Check if a custom login icon path is configured
	if Config.Frontend.LoginIcon == "" {
		Env.LoginIconPath = Env.LoginIconEmbeddedPath
		Env.LoginIconIsCustom = false
		return
	}

	// Validate custom login icon
	validatedPath, err := validateCustomImage(Config.Frontend.LoginIcon, imageName, maxSize, allowedFormats)
	if err != nil {
		logger.Warningf("Custom login icon validation failed: %v, using default", err)
		Env.LoginIconPath = Env.LoginIconEmbeddedPath
		Env.LoginIconIsCustom = false
		return
	}

	// Update to validated path and mark as custom
	Env.LoginIconPath = validatedPath
	Env.LoginIconIsCustom = true
	logger.Infof("Using custom login icon: %s", Env.LoginIconPath)
}

// setConditionalsMap builds optimized map structures from conditional rules for O(1) lookups
func setConditionals(config *Source) {

	// Merge rules from both old format (Conditionals.ItemRules) and new format (Rules)
	rules := append(config.Config.Conditionals.ItemRules, config.Config.Rules...)

	// Initialize the maps structure (only exact match maps for Names)
	resolved := ResolvedRulesConfig{
		FileNames:                make(map[string]ConditionalRule),
		FolderNames:              make(map[string]ConditionalRule),
		FilePaths:                make(map[string]ConditionalRule),
		FolderPaths:              make(map[string]ConditionalRule),
		FileEndsWith:             make([]ConditionalRule, 0),
		FolderEndsWith:           make([]ConditionalRule, 0),
		FileStartsWith:           make([]ConditionalRule, 0),
		FolderStartsWith:         make([]ConditionalRule, 0),
		NeverWatchPaths:          make(map[string]struct{}),
		IncludeRootItems:         make(map[string]struct{}),
		IgnoreAllHidden:          false,
		IgnoreAllZeroSizeFolders: false,
		IgnoreAllSymlinks:        false,
		IndexingDisabled:         false,
	}

	// backwards compatibility
	if config.Config.Conditionals.Hidden {
		logger.Warning("source.conditionals.hidden is deprecated, use source.rules instead")
		resolved.IgnoreAllHidden = true
	}
	// Backwards compatibility: if old format fields are set, treat as global rules
	if config.Config.Conditionals.IgnoreHidden {
		logger.Warning("source.conditionals.ignoreHidden is deprecated, use source.rules instead")
		resolved.IgnoreAllHidden = true
	}
	if config.Config.DisableIndexing {
		logger.Warning("source.disableIndexing is deprecated, use source.rules instead")
		resolved.IndexingDisabled = true
	}

	if config.Config.Conditionals.ZeroSizeFolders {
		logger.Warning("source.conditionals.zeroSizeFolders is deprecated, use source.rules instead")
		resolved.IgnoreAllZeroSizeFolders = true
	}

	// Process all rules and infer global flags from root-level rules
	for _, rule := range rules {
		// Check if this is a root-level rule (folderPath == "/")
		// Root-level rules with ignoreHidden/ignoreZeroSizeFolders/viewable set global flags
		isRootLevelRule := rule.FolderPath == "/"

		// Infer global flags from root-level rules
		if isRootLevelRule {
			if rule.IgnoreHidden {
				resolved.IgnoreAllHidden = true
			}
			if rule.IgnoreSymlinks {
				resolved.IgnoreAllSymlinks = true
			}
			if rule.IgnoreZeroSizeFolders {
				resolved.IgnoreAllZeroSizeFolders = true
			}
			if rule.Viewable {
				resolved.IndexingDisabled = true
			}
		}

		// Build optimized lookup structures
		if rule.FileEndsWith != "" {
			resolved.FileEndsWith = append(resolved.FileEndsWith, rule)
		}
		if rule.FolderEndsWith != "" {
			resolved.FolderEndsWith = append(resolved.FolderEndsWith, rule)
		}
		if rule.FileStartsWith != "" {
			resolved.FileStartsWith = append(resolved.FileStartsWith, rule)
		}
		if rule.FolderStartsWith != "" {
			resolved.FolderStartsWith = append(resolved.FolderStartsWith, rule)
		}
		if rule.FilePath != "" {
			resolved.FilePaths[rule.FilePath] = rule
		}
		if rule.FolderPath != "" {
			resolved.FolderPaths[rule.FolderPath] = rule
		}
		if rule.NeverWatchPath != "" {
			resolved.NeverWatchPaths[rule.NeverWatchPath] = struct{}{}
		}
		if rule.IncludeRootItem != "" {
			resolved.IncludeRootItems[rule.IncludeRootItem] = struct{}{}
		}
		if rule.FileNames != "" {
			resolved.FileNames[rule.FileNames] = rule
		}
		if rule.FolderNames != "" {
			resolved.FolderNames[rule.FolderNames] = rule
		}
		if rule.FileName != "" {
			resolved.FileNames[rule.FileName] = rule
		}
		if rule.FolderName != "" {
			resolved.FolderNames[rule.FolderName] = rule
		}
	}
	config.Config.ResolvedRules = resolved
}

func modifyExcludeInclude(config *Source) {
	// Helper to normalize a full path value (FilePaths, FolderPaths)
	// These always start with "/" and match against full index paths
	normalizeFullPath := func(value string, checkExists bool) string {
		if value == "" {
			return ""
		}
		normalized := "/" + strings.Trim(value, "/")
		// check if file/folder exists
		if checkExists {
			realPath, err := filepath.Abs(config.Path + normalized)
			if err != nil {
				logger.Warningf("could not get absolute path for %v: %v", normalized, err)
				return normalized
			}
			if _, err := os.Stat(realPath); os.IsNotExist(err) {
				logger.Warningf("configured exclude/include path %v does not exist", realPath)
			}
		}
		return normalized
	}

	// Helper to normalize a name-based value (FileNames, FolderNames, StartsWith, EndsWith)
	// These match against baseName, so no leading slash
	normalizeName := func(value string) string {
		// Just trim slashes - names shouldn't have slashes
		return strings.Trim(value, "/")
	}

	// Normalize []ConditionalRule slices for full paths
	// Handle both old format (Conditionals.ItemRules) and new format (Rules)
	allRules := append(config.Config.Conditionals.ItemRules, config.Config.Rules...)

	for i, rule := range allRules {
		// normalize full paths
		allRules[i].FilePath = normalizeFullPath(rule.FilePath, true)
		// FolderPath gets trailing slash for proper prefix matching
		if rule.FolderPath != "" {
			normalized := normalizeFullPath(rule.FolderPath, true)
			if normalized != "/" && !strings.HasSuffix(normalized, "/") {
				normalized = normalized + "/"
			}
			allRules[i].FolderPath = normalized
		}
		allRules[i].NeverWatchPath = normalizeFullPath(rule.NeverWatchPath, true)
		if rule.IncludeRootItem != "" {
			normalized := normalizeFullPath(rule.IncludeRootItem, true)
			if normalized != "/" && !strings.HasSuffix(normalized, "/") {
				normalized = normalized + "/"
			}
			allRules[i].IncludeRootItem = normalized
		}

		// normalize names
		allRules[i].FileNames = normalizeName(rule.FileNames)
		allRules[i].FolderNames = normalizeName(rule.FolderNames)
		allRules[i].FileName = normalizeName(rule.FileName)
		allRules[i].FolderName = normalizeName(rule.FolderName)
	}

	// Update the original slices with normalized values
	itemRulesLen := len(config.Config.Conditionals.ItemRules)
	config.Config.Conditionals.ItemRules = allRules[:itemRulesLen]
	config.Config.Rules = allRules[itemRulesLen:]

}
