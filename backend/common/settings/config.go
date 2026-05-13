package settings

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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
	// Migrate deprecated UserDefaults fields to new organized structure
	migrateUserDefaults()
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
	setupMedia(false)
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
	fileutils.SetFsPermissions(uint32(filePermOctal), uint32(dirPermOctal))

	// Perform mandatory cache directory speed test
	testCacheDirSpeed()

	if err := PrepareDownloadSpoolDir(); err != nil {
		logger.Fatalf("cacheDir failed to prepare download spool: %v", err)
	}

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

func setupMedia(generate bool) {
	// Save user's explicit config before applying defaults
	userImageConfig := make(map[ImagePreviewType]*bool)
	for k, v := range Config.Integrations.Media.Convert.ImagePreview {
		userImageConfig[k] = v
	}

	// Re-initialize with defaults (gets wiped out during YAML unmarshal)
	Config.Integrations.Media.Convert.ImagePreview = make(map[ImagePreviewType]*bool)
	Config.Integrations.Media.Convert.ImagePreview[HEICImagePreview] = boolPtr(false) // HEIC defaults to disabled
	Config.Integrations.Media.Convert.ImagePreview[JPEGImagePreview] = boolPtr(true)  // JPEG fallback defaults to enabled

	// Apply user overrides (only for keys explicitly set in YAML)
	for k, v := range userImageConfig {
		Config.Integrations.Media.Convert.ImagePreview[k] = v
	}

	// VideoPreview: Merge user config with defaults
	// Save user's explicit config from YAML
	userVideoConfig := make(map[VideoPreviewType]*bool)
	for k, v := range Config.Integrations.Media.Convert.VideoPreview {
		userVideoConfig[k] = v
	}

	// Re-initialize with defaults (all enabled)
	Config.Integrations.Media.Convert.VideoPreview = make(map[VideoPreviewType]*bool)
	for _, t := range AllVideoPreviewTypes {
		Config.Integrations.Media.Convert.VideoPreview[t] = boolPtr(true)
	}

	// Apply user overrides (only for keys explicitly set in YAML)
	for k, v := range userVideoConfig {
		Config.Integrations.Media.Convert.VideoPreview[k] = v
	}

	// Resolve exiftool path once at startup: validate user path or discover via PATH
	if Config.Integrations.Media.ExiftoolPath != "" && !generate {
		if err := exec.Command(Config.Integrations.Media.ExiftoolPath, "-ver").Run(); err != nil {
			logger.Warningf("exiftool path is invalid or not executable: %q (%v); disabling exiftool", Config.Integrations.Media.ExiftoolPath, err)
			Config.Integrations.Media.ExiftoolPath = ""
		}
	}
	if Config.Integrations.Media.ExiftoolPath == "" && !generate {
		if path, err := exec.LookPath("exiftool"); err == nil && path != "" {
			Config.Integrations.Media.ExiftoolPath = path
		}
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
			source.Path = realPath // use absolute path
			if source.Name == "" {
				_, ok := Config.Server.SourceMap[source.Path]
				if ok {
					source.Name = name + fmt.Sprintf("-%v", k)
				} else {
					source.Name = name
				}
			}
			if generate {
				source.Path = generatorPath // use placeholder path
				source.Name = "Source Name"
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
	if Config.Auth.Methods.NoAuth {
		logger.Warning("Configured with no authentication, this is not recommended.")
		Config.Auth.AuthMethods = []string{"disabled"}
	}
	if Config.Auth.Methods.OidcAuth.Enabled || generate {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "oidc")
		err := validateOidcAuth()
		if err != nil && !generate {
			logger.Fatalf("Error validating OIDC auth: %v", err)
		}
		logger.Info("OIDC Auth configured successfully")
	}
	if Config.Auth.Methods.LdapAuth.Enabled || generate {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "ldap")
		err := ValidateLdapAuth()
		if err != nil && !generate {
			logger.Fatalf("Error validating LDAP auth: %v", err)
		}
		logger.Info("LDAP Auth configured successfully")
	}
	if Config.Auth.Methods.JwtAuth.Enabled || generate {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "jwt")
		err := ValidateJwtAuth()
		if err != nil && !generate {
			logger.Fatalf("Error validating JWT auth: %v", err)
		}
		logger.Info("JWT Auth configured successfully")
	}
	if Config.Auth.Methods.PasskeyAuth.Enabled || generate {
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "passkey")
		logger.Info("Passkey Auth configured successfully")
	}

	// use password auth as default if no auth methods are set
	if len(Config.Auth.AuthMethods) == 0 {
		Config.Auth.Methods.PasswordAuth.Enabled = true
		Config.Auth.AuthMethods = append(Config.Auth.AuthMethods, "password")
	}
	Config.UserDefaults.Account.LoginMethod = Config.Auth.AuthMethods[0]

}

func setupLogging() {
	if len(Config.Server.Logging) == 0 {
		Config.Server.Logging = []LogConfig{
			{
				Output: "stdout",
			},
		}
	}
	for i := range Config.Server.Logging {
		cfg := &Config.Server.Logging[i]
		// Enable debug logging automatically in dev mode
		levels := cfg.Levels
		if os.Getenv("FILEBROWSER_DEVMODE") == "true" {
			levels = "info|warning|error|debug"
		}
		pattern := cfg.ApiFilter
		if pattern == "" {
			pattern = "^/health|^/favicon.ico|^/static|^/public/static"
		}
		jsonCfg := logger.JsonConfig{
			Levels:         levels,
			ApiLevels:      cfg.ApiLevels,
			Output:         cfg.Output,
			Utc:            cfg.Utc,
			NoColors:       cfg.NoColors,
			Json:           cfg.Json,
			Structured:     false,
			ApiPathExclude: pattern,
		}
		err := logger.EnableCompatibilityMode(jsonCfg)
		if err != nil {
			log.Println("[ERROR] Failed to set up logger:", err)
		}
	}
}

// migrateUserDefaults migrates deprecated UserDefaults fields to the new organized structure.
// This function maintains backwards compatibility by checking if new fields are unset and copying
// values from old deprecated fields if found. Logs warnings when old fields are migrated.
func migrateUserDefaults() {
	ud := &Config.UserDefaults
	hasOldFields := false

	// Helper function to check if a bool pointer is unset (nil)
	isUnsetBoolPtr := func(v *bool) bool {
		return v == nil
	}

	// Helper function to check if a string is unset (empty)
	isUnsetString := func(v string) bool {
		return v == ""
	}

	// Helper function to check if an int is unset (zero)
	isUnsetInt := func(v int) bool {
		return v == 0
	}

	// Migrate Preview-related deprecated fields
	if !ud.Sidebar.DisableHideOnPreview && ud.Preview.DisableHideSidebar {
		ud.Sidebar.DisableHideOnPreview = ud.Preview.DisableHideSidebar
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'preview.disableHideSidebar' to 'sidebar.disableHideOnPreview'")
	}

	if !ud.FileViewer.DefaultMediaPlayer && ud.Preview.DefaultMediaPlayer {
		ud.FileViewer.DefaultMediaPlayer = ud.Preview.DefaultMediaPlayer
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'preview.defaultMediaPlayer' to 'fileViewer.defaultMediaPlayer'")
	}

	if isUnsetBoolPtr(ud.FileViewer.AutoplayMedia) && ud.Preview.AutoplayMedia {
		autoplay := ud.Preview.AutoplayMedia
		ud.FileViewer.AutoplayMedia = &autoplay
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'preview.autoplayMedia' to 'fileViewer.autoplayMedia'")
	}

	if isUnsetString(ud.Preview.DisablePreviewExt) && !isUnsetString(ud.DisablePreviewExt) {
		ud.Preview.DisablePreviewExt = ud.DisablePreviewExt
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'disablePreviewExt' to 'preview.disablePreviewExt'")
	}

	// Migrate Sidebar fields
	if !ud.Sidebar.DisableQuickToggles && ud.DisableQuickToggles {
		ud.Sidebar.DisableQuickToggles = ud.DisableQuickToggles
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'disableQuickToggles' to 'sidebar.disableQuickToggles'")
	}

	if !ud.Sidebar.HideFileActions && ud.HideSidebarFileActions {
		ud.Sidebar.HideFileActions = ud.HideSidebarFileActions
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'hideSidebarFileActions' to 'sidebar.hideFileActions'")
	}

	if !ud.Sidebar.Sticky && ud.StickySidebar {
		ud.Sidebar.Sticky = ud.StickySidebar
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'stickySidebar' to 'sidebar.sticky'")
	}

	if isUnsetString(ud.Listing.ViewMode) && !isUnsetString(ud.ViewMode) {
		ud.Listing.ViewMode = ud.ViewMode
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'viewMode' to 'sidebar.viewMode'")
	}

	if isUnsetInt(ud.Listing.GallerySize) && !isUnsetInt(ud.GallerySize) {
		ud.Listing.GallerySize = ud.GallerySize
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'gallerySize' to 'sidebar.gallerySize'")
	}

	if !ud.Sidebar.HideFiles && ud.HideFilesInTree {
		ud.Sidebar.HideFiles = ud.HideFilesInTree
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'hideFilesInTree' to 'sidebar.hideFiles'")
	}

	if isUnsetBoolPtr(ud.Sidebar.ShowTools) && ud.ShowToolsInSidebar != nil {
		ud.Sidebar.ShowTools = ud.ShowToolsInSidebar
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'showToolsInSidebar' to 'sidebar.showTools'")
	}

	// Migrate Listing fields
	if !ud.Listing.DeleteWithoutConfirming && ud.DeleteWithoutConfirming {
		ud.Listing.DeleteWithoutConfirming = ud.DeleteWithoutConfirming
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'deleteWithoutConfirming' to 'listing.deleteWithoutConfirming'")
	}

	if !ud.Listing.DateFormat && ud.DateFormat {
		ud.Listing.DateFormat = ud.DateFormat
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'dateFormat' to 'listing.dateFormat'")
	}

	if !ud.Listing.ShowHidden && ud.ShowHidden {
		ud.Listing.ShowHidden = ud.ShowHidden
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'showHidden' to 'listing.showHidden'")
	}

	if !ud.Listing.QuickDownload && ud.QuickDownload {
		ud.Listing.QuickDownload = ud.QuickDownload
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'quickDownload' to 'listing.quickDownload'")
	}

	if !ud.Listing.ShowSelectMultiple && ud.ShowSelectMultiple {
		ud.Listing.ShowSelectMultiple = ud.ShowSelectMultiple
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'showSelectMultiple' to 'listing.showSelectMultiple'")
	}

	if !ud.Listing.SingleClick && ud.SingleClick {
		ud.Listing.SingleClick = ud.SingleClick
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'singleClick' to 'listing.singleClick'")
	}

	if isUnsetString(ud.Listing.HideFileExt) && !isUnsetString(ud.HideFileExt) {
		ud.Listing.HideFileExt = ud.HideFileExt
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'hideFileExt' to 'listing.hideFileExt'")
	}

	if !ud.Listing.ShowCopyPath && ud.ShowCopyPath {
		ud.Listing.ShowCopyPath = ud.ShowCopyPath
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'showCopyPath' to 'listing.showCopyPath'")
	}

	if !ud.Listing.DeleteAfterArchive && ud.DeleteAfterArchive {
		ud.Listing.DeleteAfterArchive = ud.DeleteAfterArchive
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'deleteAfterArchive' to 'listing.deleteAfterArchive'")
	}

	// Migrate FileViewer fields
	if !ud.FileViewer.EditorQuickSave && ud.EditorQuickSave {
		ud.FileViewer.EditorQuickSave = ud.EditorQuickSave
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'editorQuickSave' to 'fileViewer.editorQuickSave'")
	}

	if isUnsetString(ud.FileViewer.DisableViewingExt) && !isUnsetString(ud.DisableViewingExt) {
		ud.FileViewer.DisableViewingExt = ud.DisableViewingExt
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'disableViewingExt' to 'fileViewer.disableViewingExt'")
	}

	if isUnsetString(ud.FileViewer.DisableOnlyOfficeExt) && !isUnsetString(ud.DisableOnlyOfficeExt) {
		ud.FileViewer.DisableOnlyOfficeExt = ud.DisableOnlyOfficeExt
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'disableOnlyOfficeExt' to 'fileViewer.disableOnlyOfficeExt'")
	}

	if !ud.FileViewer.PreferEditorForMarkdown && ud.PreferEditorForMarkdown {
		ud.FileViewer.PreferEditorForMarkdown = ud.PreferEditorForMarkdown
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'preferEditorForMarkdown' to 'fileViewer.preferEditorForMarkdown'")
	}

	if !ud.FileViewer.DebugOffice && ud.DebugOffice {
		ud.FileViewer.DebugOffice = ud.DebugOffice
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'debugOffice' to 'fileViewer.debugOffice'")
	}

	// Migrate Search fields
	if !ud.Search.DisableOptions && ud.DisableSearchOptions {
		ud.Search.DisableOptions = ud.DisableSearchOptions
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'disableSearchOptions' to 'search.disableOptions'")
	}

	// Migrate UI fields
	if isUnsetBoolPtr(ud.UI.DarkMode) && ud.DarkMode != nil {
		ud.UI.DarkMode = ud.DarkMode
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'darkMode' to 'ui.darkMode'")
	}

	if isUnsetString(ud.UI.ThemeColor) && !isUnsetString(ud.ThemeColor) {
		ud.UI.ThemeColor = ud.ThemeColor
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'themeColor' to 'ui.themeColor'")
	}

	if isUnsetString(ud.UI.CustomTheme) && !isUnsetString(ud.CustomTheme) {
		ud.UI.CustomTheme = ud.CustomTheme
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'customTheme' to 'ui.customTheme'")
	}

	if isUnsetString(ud.UI.Locale) && !isUnsetString(ud.Locale) {
		ud.UI.Locale = ud.Locale
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'locale' to 'ui.locale'")
	}

	// Migrate Account fields
	if !ud.Account.LockPassword && ud.LockPassword {
		ud.Account.LockPassword = ud.LockPassword
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'lockPassword' to 'account.lockPassword'")
	}

	if !ud.Account.DisableSettings && ud.DisableSettings {
		ud.Account.DisableSettings = ud.DisableSettings
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'disableSettings' to 'account.disableSettings'")
	}

	if isUnsetString(ud.Account.LoginMethod) && !isUnsetString(ud.LoginMethod) {
		ud.Account.LoginMethod = ud.LoginMethod
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'loginMethod' to 'account.loginMethod'")
	}

	if !ud.Account.DisableUpdateNotifications && ud.DisableUpdateNotifications {
		ud.Account.DisableUpdateNotifications = ud.DisableUpdateNotifications
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'disableUpdateNotifications' to 'account.disableUpdateNotifications'")
	}

	// Migrate Account Permissions fields
	if !ud.Account.Permissions.Api && ud.Permissions.Api {
		ud.Account.Permissions.Api = ud.Permissions.Api
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'permissions.api' to 'account.permissions.api'")
	}

	if !ud.Account.Permissions.Admin && ud.Permissions.Admin {
		ud.Account.Permissions.Admin = ud.Permissions.Admin
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'permissions.admin' to 'account.permissions.admin'")
	}

	if !ud.Account.Permissions.Modify && ud.Permissions.Modify {
		ud.Account.Permissions.Modify = ud.Permissions.Modify
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'permissions.modify' to 'account.permissions.modify'")
	}

	if !ud.Account.Permissions.Share && ud.Permissions.Share {
		ud.Account.Permissions.Share = ud.Permissions.Share
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'permissions.share' to 'account.permissions.share'")
	}

	if !ud.Account.Permissions.Realtime && ud.Permissions.Realtime {
		ud.Account.Permissions.Realtime = ud.Permissions.Realtime
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'permissions.realtime' to 'account.permissions.realtime'")
	}

	if !ud.Account.Permissions.Delete && ud.Permissions.Delete {
		ud.Account.Permissions.Delete = ud.Permissions.Delete
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'permissions.delete' to 'account.permissions.delete'")
	}

	if !ud.Account.Permissions.Create && ud.Permissions.Create {
		ud.Account.Permissions.Create = ud.Permissions.Create
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'permissions.create' to 'account.permissions.create'")
	}

	if isUnsetBoolPtr(ud.Account.Permissions.Download) && ud.Permissions.Download != nil {
		ud.Account.Permissions.Download = ud.Permissions.Download
		hasOldFields = true
		logger.Warning("userDefaults: migrating deprecated field 'permissions.download' to 'account.permissions.download'")
	}

	if hasOldFields {
		logger.Warning("userDefaults: Please update your configuration to use the new organized structure. See documentation for details.")
	}
}

func loadConfigWithDefaults(configFile string, generate bool) error {
	Config = SetDefaults(generate)

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

	ldapUserPassword := os.Getenv("FILEBROWSER_LDAP_USER_PASSWORD")
	if ldapUserPassword != "" {
		Config.Auth.Methods.LdapAuth.UserPassword = ldapUserPassword
		logger.Info("Using LDAP bind password from FILEBROWSER_LDAP_USER_PASSWORD environment variable")
	}
}

func SetDefaults(generate bool) Settings {
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
	if _, err := os.Stat(database); os.IsNotExist(err) {
		logger.Warning("database file could not be found. If this is unexpected, please set the FILEBROWSER_DATABASE environment variable to the correct path.")
	}
	s := Settings{
		Server: Server{
			Port:               80,
			NumImageProcessors: numCpus,
			BaseURL:            "",
			Database:           database,
			SourceMap:          map[string]*Source{},
			NameToSource:       map[string]*Source{},
			CacheDir:           "tmp",
			MaxArchiveSizeGB:   20,
			IndexSqlConfig: IndexSqlConfig{
				WalMode:               false,
				BatchSize:             1000,
				CacheSizeMB:           32,
				DisableReuse:          false,
				StartupIntegrityCheck: IndexStartupIntegrityQuickCheck,
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
			// New organized structure
			Sidebar: UserDefaultsSidebar{
				DisableQuickToggles:  false,
				HideFileActions:      false,
				DisableHideOnPreview: false,
				Sticky:               true,
				HideFiles:            false,
				ShowTools:            boolPtr(true),
			},
			Listing: UserDefaultsListing{
				DeleteWithoutConfirming: false,
				DateFormat:              false,
				ShowHidden:              false,
				QuickDownload:           false,
				ShowSelectMultiple:      false,
				SingleClick:             false,
				HideFileExt:             "",
				ShowCopyPath:            false,
				DeleteAfterArchive:      true,
				ViewMode:                "normal",
				GallerySize:             3,
			},
			Preview: UserDefaultsPreview{
				Image:              boolPtr(true),
				Video:              boolPtr(true),
				Audio:              boolPtr(true),
				MotionVideoPreview: boolPtr(true),
				Office:             boolPtr(true),
				PopUp:              boolPtr(true),
				DisablePreviewExt:  "",
				HighQuality:        boolPtr(true),
				Folder:             boolPtr(true),
				Models:             boolPtr(true),
			},
			FileViewer: UserDefaultsFileViewer{
				EditorQuickSave:         false,
				AutoplayMedia:           boolPtr(true),
				DisableViewingExt:       "",
				DisableOnlyOfficeExt:    ".md .txt .pdf .html .xml",
				PreferEditorForMarkdown: false,
				DebugOffice:             false,
				DefaultMediaPlayer:      false,
			},
			Search: UserDefaultsSearch{
				DisableOptions: false,
			},
			UI: UserDefaultsUI{
				DarkMode:    boolPtr(true),
				ThemeColor:  "var(--blue)",
				CustomTheme: "",
				Locale:      "en",
			},
			FileLoading: users.FileLoading{
				MaxConcurrent:     10,
				UploadChunkSize:   10, // 10MB
				DownloadChunkSize: 0,  // 0MB, default to no chunking
			},
			Account: UserDefaultsAccount{
				Permissions: UserDefaultsAccountPermissions{
					Api:      false,
					Admin:    false,
					Modify:   false,
					Share:    false,
					Realtime: false,
					Delete:   false,
					Create:   false,
					Download: boolPtr(true), // defaults to true
				},
				LockPassword:               false,
				DisableSettings:            false,
				LoginMethod:                "",
				DisableUpdateNotifications: false,
			},
		},
	}
	// Initialize ImagePreview map with defaults
	s.Integrations.Media.Convert.ImagePreview = make(map[ImagePreviewType]*bool)
	s.Integrations.Media.Convert.ImagePreview[HEICImagePreview] = boolPtr(false) // HEIC defaults to disabled
	s.Integrations.Media.Convert.ImagePreview[JPEGImagePreview] = boolPtr(true)  // JPEG fallback defaults to enabled
	// Initialize VideoPreview map with defaults (all enabled)
	s.Integrations.Media.Convert.VideoPreview = make(map[VideoPreviewType]*bool)
	for _, t := range AllVideoPreviewTypes {
		s.Integrations.Media.Convert.VideoPreview[t] = boolPtr(true)
	}
	return s
}

// validateCustomImage validates a custom image file path and returns the absolute path or error
func validateCustomImage(configPath, imageName string, allowedFormats []string) (absolutePath string, err error) {
	// Get absolute path
	absolutePath, err = filepath.Abs(configPath)
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %w", err)
	}

	// Check if file exists
	_, err = os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("could not access file: %w", err)
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
	allowedFormats := []string{".ico", ".png", ".svg", ".jpg", ".jpeg", ".gif", ".webp"}

	// Set default embedded favicon path
	Env.FaviconEmbeddedPath = "img/icons/favicon.svg"

	// Check if a custom favicon path is configured
	if Config.Frontend.Favicon == "" {
		Env.FaviconPath = Env.FaviconEmbeddedPath
		Env.FaviconIsCustom = false
		return
	}

	// Validate custom favicon
	validatedPath, err := validateCustomImage(Config.Frontend.Favicon, imageName, allowedFormats)
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
	// PWA and platform PNGs are generated at server startup from this path (SVG uses a raster
	// sidecar next to the .svg when present; see icons.GeneratePWAIcons).
}

// PWAIconsCacheDir is where startup icon generation writes PNGs (under the server cache dir).
func PWAIconsCacheDir() string {
	return filepath.Join(Config.Server.CacheDir, "icons")
}

// Where transient archives for multi-request (Range) downloads are stored to support chunked downloads.
func DownloadCacheDir() string {
	return filepath.Join(Config.Server.CacheDir, "downloads")
}

// PrepareDownloadSpoolDir creates the download spool directory and removes any leftover dl-archive-*
func PrepareDownloadSpoolDir() error {
	if err := os.MkdirAll(DownloadCacheDir(), fileutils.PermDir); err != nil {
		return err
	}
	return fileutils.ClearDirectoryContents(DownloadCacheDir())
}

func loadLoginIcon() {
	const imageName = "login icon"
	allowedFormats := []string{".svg", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico"}

	// Set default embedded icon path - just use favicon.svg (light/dark handled by CSS)
	Env.LoginIconEmbeddedPath = "img/icons/favicon.svg"

	// Check if a custom login icon path is configured
	if Config.Frontend.LoginIcon == "" {
		Env.LoginIconPath = Env.LoginIconEmbeddedPath
		Env.LoginIconIsCustom = false
		return
	}

	// Validate custom login icon
	validatedPath, err := validateCustomImage(Config.Frontend.LoginIcon, imageName, allowedFormats)
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
		NoRules:                  len(rules) == 0,
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
