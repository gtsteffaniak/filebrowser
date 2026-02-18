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

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	fbhttp "github.com/gtsteffaniak/filebrowser/backend/http"
	"github.com/gtsteffaniak/filebrowser/backend/icons"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/preview"
	"github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/swaggo/swag"
)

var store *bolt.BoltStore

func getStore(configFile string) bool {
	// Use the config file (global flag)
	settings.Initialize(configFile)
	s, hasDB, err := storage.InitializeDb(settings.Config.Server.Database)
	if err != nil {
		logger.Fatalf("could not load db info: %v", err)
	}
	store = s
	return hasDB
}

func generalUsage() {
	fmt.Printf(`usage: ./filebrowser <command> [options]
commands:
	-h    	Print help
	-c    	Print the default config file
	version Print version information
	set -u	Username and password for the new user
	set -a	Create user as admin
	set -s	Specify a user scope
	set -h	Print this help message
`)
}

func StartFilebrowser() {
	keepGoing := runCLI()
	if !keepGoing {
		return
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
	dbExists := getStore(configPath)
	database := fmt.Sprintf("Using existing database  : %v", settings.Config.Server.Database)
	if !dbExists {
		database = fmt.Sprintf("Creating new database    : %v", settings.Config.Server.Database)
	}

	// Dev mode enables development features like template hot-reloading
	_, err := os.Stat("http/dist")
	// In dev mode, always use filesystem assets. Otherwise, check if http/dist exists
	if !settings.Env.IsDevMode {
		settings.Env.EmbeddedFs = os.IsNotExist(err)
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
	if store != nil && store.Indexing != nil {
		indexing.SetIndexingStorage(store.Indexing)
		if isNewDb {
			if err := store.Indexing.ResetAllComplexities(); err != nil {
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
		if err := rootCMD(ctx, store, &serverConfig, shutdownComplete); err != nil {
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
	logger.Info("Shutdown complete.")
}

func rootCMD(ctx context.Context, store *bolt.BoltStore, serverConfig *settings.Server, shutdownComplete chan struct{}) error {
	if serverConfig.NumImageProcessors < 1 {
		logger.Fatal("Image resize workers count could not be < 1")
	}
	cacheDir := settings.Config.Server.CacheDir
	numWorkers := settings.Config.Server.NumImageProcessors
	ffmpeg.SetFFmpegPaths()
	
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
	
	// Generate PWA icons after preview service is initialized
	if err := icons.GeneratePWAIcons(); err != nil {
		logger.Warningf("Failed to generate PWA icons: %v", err)
	}
	
	// Initialize PWA manifest after icons are generated
	icons.InitializePWAManifest()
	
	fbhttp.StartHttp(ctx, store, shutdownComplete)
	return nil
}
