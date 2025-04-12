package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gtsteffaniak/filebrowser/backend/diskcache"
	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/fileutils"
	fbhttp "github.com/gtsteffaniak/filebrowser/backend/http"
	"github.com/gtsteffaniak/filebrowser/backend/img"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
	"github.com/swaggo/swag"

	"github.com/gtsteffaniak/filebrowser/backend/version"
)

var store *storage.Storage

func getStore(configFile string) bool {
	// Use the config file (global flag)
	settings.Initialize(configFile)
	s, hasDB, err := storage.InitializeDb(settings.Config.Server.Database)
	if err != nil {
		logger.Fatal(fmt.Sprintf("could not load db info: %v", err))
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
	sourceList := []string{}
	for path, source := range settings.Config.Server.SourceMap {
		sourceList = append(sourceList, fmt.Sprintf("%v: %v", source.Name, path))
	}
	logger.Info(fmt.Sprintf("Initializing FileBrowser Quantum (%v)", version.Version))
	logger.Info(fmt.Sprintf("Using Config file        : %v", configPath))
	logger.Info(fmt.Sprintf("Auth Methods             : %v", settings.Config.Auth.AuthMethods))
	logger.Info(database)
	logger.Info(fmt.Sprintf("Sources                  : %v", sourceList))
	serverConfig := settings.Config.Server
	swagInfo := docs.SwaggerInfo
	swagInfo.BasePath = serverConfig.BaseURL
	swag.Register(docs.SwaggerInfo.InstanceName(), swagInfo)
	// initialize indexing and schedule indexing ever n minutes (default 5)
	if len(settings.Config.Server.SourceMap) == 0 {
		logger.Fatal("No sources configured, exiting...")
	}
	for _, source := range settings.Config.Server.SourceMap {
		go files.Initialize(source)
	}
	validateUserInfo()
	// Start the rootCMD in a goroutine
	go func() {
		if err := rootCMD(ctx, store, &serverConfig, shutdownComplete); err != nil {
			logger.Fatal(fmt.Sprintf("Error starting filebrowser: %v", err))
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
	fileutils.ClearCacheDir()

	<-shutdownComplete // Ensure we don't exit prematurely
	// Wait for the server to stop
	logger.Info("Shutdown complete.")
}

func rootCMD(ctx context.Context, store *storage.Storage, serverConfig *settings.Server, shutdownComplete chan struct{}) error {
	if serverConfig.NumImageProcessors < 1 {
		logger.Fatal("Image resize workers count could not be < 1")
	}
	imgSvc := img.New(serverConfig.NumImageProcessors)

	cacheDir := settings.Config.Server.CacheDir
	var fileCache diskcache.Interface

	// Use file cache if cacheDir is specified
	if cacheDir != "" {
		var err error
		fileCache, err = diskcache.NewFileCache(cacheDir)
		if err != nil {
			if cacheDir == "tmp" {
				logger.Error("The cache dir could not be created. Make sure the user that you executed the program with has access to create directories in the local path. filebrowser is trying to use the default `server.cacheDir: tmp` , but you can change this location if you need to. Please see configuration wiki for more information about this error. https://github.com/gtsteffaniak/filebrowser/wiki/Configuration")
			}
			logger.Fatal(fmt.Sprintf("failed to create file cache path, which is now require to run the server: %v", err))
		}
	} else {
		// No-op cache if no cacheDir is specified
		fileCache = diskcache.NewNoOp()
	}
	fbhttp.StartHttp(ctx, imgSvc, store, fileCache, shutdownComplete)

	return nil
}
