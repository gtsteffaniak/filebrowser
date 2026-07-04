package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/version"
	fbhttp "github.com/gtsteffaniak/filebrowser/backend/internal/web"
	"github.com/gtsteffaniak/filebrowser/backend/internal/icons"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/internal/preview"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/swaggo/swag"
)

func initializeDatabase(configFile string) bool {
	// Use the config file (global flag)
	settings.Initialize(configFile)

	// Check if migration is needed
	if checkMigrationNeeded() {
		logger.Info("Old BoltDB database detected, starting migration to SQLite...")
		err := migrateFromBoltToSQLite()
		if err != nil {
			logger.Fatalf("Migration failed: %v", err)
		}
	}

	// Initialize state management system
	existingDb, err := state.Initialize(settings.Config.Server.DatabaseV2.Path)
	if err != nil {
		logger.Fatalf("could not initialize state: %v", err)
	}

	return existingDb
}

func generalUsage() {
	fmt.Printf(`usage: ./filebrowser <command> [options]
commands:
	-h    		Print help
	-c    		Path to config file (global default: config.yaml)
	version 	Print version information
	setup   		Interactive config setup
	set -u 		Username and password: set -u <user>,<password> [-c config.yaml]
	set rule	Access rules: set rule -s <sourceName> -p <indexPath> -r user|group|all -v <name> [-allow] [-c config.yaml] (-sourceName/-sourcePath same as -s/-p)
`)
}

func StartFilebrowser() {
	keepGoing, dbExists := runCLI()
	if !keepGoing {
		return
	}
	database := fmt.Sprintf("Using existing database  : %v", settings.Config.Server.DatabaseV2.Path)
	if !dbExists {
		database = fmt.Sprintf("Creating new database    : %v", settings.Config.Server.DatabaseV2.Path)
	}
	if !settings.Config.Server.DisableUpdateCheck {
		info, _ := utils.CheckForUpdates()
		if info.LatestVersion != "" {
			logger.Infof("A new version is available: %s (current: %s)", info.LatestVersion, info.CurrentVersion)
			logger.Infof("Release notes: %s", info.ReleaseNotes)
		}
		go utils.StartCheckForUpdates()
	}

	// Create context and channels for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	done := make(chan struct{})             // Signals server has stopped
	shutdownComplete := make(chan struct{}) // Signals shutdown process is complete

	// Dev mode enables development features like template hot-reloading
	_, err := os.Stat("internal/web/dist")
	// In dev mode, always use filesystem assets. Otherwise, check if internal/web/dist exists
	if !settings.Env.IsDevMode {
		settings.Env.EmbeddedFs = os.IsNotExist(err)
	}

	err = utils.SetInvalidPasswordHash()
	if err != nil {
		logger.Fatalf("Failed to set security hash: %v", err)
	}

	sourceList := []string{}
	for path, source := range settings.Config.Server.SourceMap {
		sourceList = append(sourceList, fmt.Sprintf("%v: %v", source.Name, path))
	}
	logger.Infof("Initializing FileBrowser Quantum (%v)", version.Version)
	logger.Infof("Using Config file        : %v", configPath)
	logger.Infof("Auth Methods             : %v", settings.Config.Auth.AuthMethods)
	logger.Info(database)
	logger.Infof("Sources                  : %v", sourceList)
	logger.Debugf("Using Embedded FS        : %v", settings.Env.EmbeddedFs)
	walModeStr := "OFF"
	if settings.Config.Server.IndexSqlConfig.WalMode {
		walModeStr = "WAL"
	}
	logger.Infof("SQL Journal Mode         : %v", walModeStr)
	if settings.Config.Server.CacheDirCleanup {
		logger.Debugf("clearing cache dir: %s", settings.Config.Server.CacheDir)
		fileutils.ClearCacheDir(settings.Config.Server.CacheDir)
	}
	serverConfig := settings.Config.Server
	swagInfo := docs.SwaggerInfo
	swagInfo.BasePath = serverConfig.BaseURL
	swag.Register(docs.SwaggerInfo.InstanceName(), swagInfo)
	// initialize indexing and schedule indexing ever n minutes (default 5)
	if len(settings.Config.Server.SourceMap) == 0 {
		logger.Fatal("No sources configured, exiting...")
	}

	// Initialize shared index database before starting HTTP service
	isNewDb, err := indexing.InitializeIndexDB()
	if err != nil {
		logger.Fatalf("Failed to initialize index database: %v", err)
	}

	// Set indexing storage for persistence
	indexingStorage := state.GetIndexingStorage()
	if indexingStorage != nil {
		indexing.SetIndexingStorage(indexingStorage)
		if isNewDb {
			if err := indexingStorage.ResetAllComplexities(); err != nil {
				logger.Errorf("Failed to reset index complexities: %v", err)
			}
		}
	}

	for _, source := range settings.Config.Server.SourceMap {
		go indexing.Initialize(source, false, isNewDb)
	}
	validateUserInfo(!dbExists)
	validateOfficeIntegration()
	validateAccessRules()
	validateShareInfo()
	// Start the rootCMD in a goroutine
	go func() {
		if err := rootCMD(ctx, &serverConfig, shutdownComplete); err != nil {
			logger.Fatalf("Error starting filebrowser: %v", err)
		}
		close(done) // Signal that the server has stopped
	}()
	// Wait for a shutdown signal or the server to stop
	select {
	case <-signalChan:
		logger.Info("Received shutdown signal. Shutting down gracefully...")
		cancel() // Trigger context cancellation
	case <-done:
		logger.Info("Server stopped unexpectedly. Shutting down...")
	}

	// Stop all indexing scanners before closing the database
	indexing.StopAllScanners()

	// Give scanners a moment to finish their current scan operations
	time.Sleep(100 * time.Millisecond)

	// cleanup temp databases
	indexDB := indexing.GetIndexDB()
	if indexDB != nil {
		indexDB.Close()
	}
	if settings.Config.Server.CacheDirCleanup {
		logger.Debugf("clearing cache dir: %s", settings.Config.Server.CacheDir)
		fileutils.ClearCacheDir(settings.Config.Server.CacheDir)
	}
	<-shutdownComplete
	if err := fileutils.ClearDirectoryContents(settings.DownloadCacheDir()); err != nil {
		logger.Warningf("failed to clear download spool on shutdown: %v", err)
	}
	logger.Info("Shutdown complete.")
}

func rootCMD(ctx context.Context, serverConfig *settings.Server, shutdownComplete chan struct{}) error {
	if serverConfig.NumImageProcessors < 1 {
		logger.Fatal("Image resize workers count could not be < 1")
	}
	cacheDir := settings.Config.Server.CacheDir
	numWorkers := settings.Config.Server.NumImageProcessors

	// Initialize asset filesystem before starting services
	if settings.Env.EmbeddedFs {
		embeddedAssets := fbhttp.GetEmbeddedAssets()
		subAssets, err := fs.Sub(embeddedAssets, "embed")
		if err != nil {
			logger.Fatalf("Failed to create sub filesystem: %v", err)
		}
		fileutils.InitAssetFS(subAssets, true)
	} else {
		fileutils.InitAssetFS(nil, false)
	}

	// Start preview service
	err := preview.StartPreviewGenerator(numWorkers, cacheDir)
	if err != nil {
		logger.Fatalf("Error starting preview service: %v", err)
	}
	logger.Debugf("MuPDF Enabled            : %v", settings.Env.MuPdfAvailable)
	logger.Debugf("Media Enabled            : %v", settings.MediaEnabled())
	logger.Debugf("Exiftool Enabled         : %v", settings.Config.Integrations.Media.ExiftoolPath != "")

	// Generate PWA icons after preview service is initialized
	if err := icons.GeneratePWAIcons(); err != nil {
		logger.Warningf("Failed to generate PWA icons: %v", err)
	}

	// Initialize PWA manifest after icons are generated
	icons.InitializePWAManifest()

	// Initialize WebAuthn/Passkey service if enabled
	if err := auth.InitWebAuthn(); err != nil {
		logger.Fatalf("Failed to initialize WebAuthn: %v", err)
	}

	fbhttp.StartHttp(ctx, shutdownComplete)
	return nil
}
