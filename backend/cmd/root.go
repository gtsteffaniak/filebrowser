package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gtsteffaniak/filebrowser/backend/diskcache"
	"github.com/gtsteffaniak/filebrowser/backend/files"
	fbhttp "github.com/gtsteffaniak/filebrowser/backend/http"
	"github.com/gtsteffaniak/filebrowser/backend/img"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
	"github.com/swaggo/swag"

	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/version"
)

func getStore(configFile string) (*storage.Storage, bool) {
	// Use the config file (global flag)
	settings.Initialize(configFile)
	store, hasDB, err := storage.InitializeDb(settings.Config.Server.Database)
	if err != nil {
		logger.Fatal(fmt.Sprintf("could not load db info: %v", err))
	}
	// update source info for users if names/sources/paths might have changed
	usersList, err2 := store.Users.Gets()
	if err2 != nil {
		logger.Fatal(fmt.Sprintf("could not load users: %v", err2))
	}

	// this function adds scopes as needed on startup
	for _, user := range usersList {
		updateUser := false
		newScopes := []users.SourceScope{}
		for _, source := range settings.Config.Server.SourceMap {
			scopePath := ""
			if !user.Perm.Admin {
				scopePath = source.Config.DefaultUserScope
			}
			scope, err := settings.GetScopeFromSourcePath(user.Scopes, source.Path)
			if err != nil {
				if user.Perm.Admin || source.Config.DefaultEnabled {
					newScopes = append(newScopes, users.SourceScope{Scope: scopePath, Name: source.Path}) // backend name is path
					updateUser = true
				}
			} else {
				newScopes = append(newScopes, users.SourceScope{Scope: scope, Name: source.Path}) // backend name is path
			}
		}
		user.Scopes = newScopes
		// maintain backwards compatibility, update user scope from scopes
		if len(user.Scopes) == 0 {
			user.Scopes = []users.SourceScope{
				{
					Scope: user.Scope,
					Name:  settings.Config.Server.DefaultSource.Path, // backend name is path
				},
			}
			updateUser = true
		}
		if !updateUser {
			continue
		}
		err := store.Users.Save(user, false)
		if err != nil {
			logger.Error(fmt.Sprintf("could not update user: %v", err))
		}
	}
	return store, hasDB
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
	// Global flags
	var configPath string
	var help bool
	// Override the default usage output to use generalUsage()
	flag.Usage = generalUsage
	flag.StringVar(&configPath, "c", "config.yaml", "Path to the config file, default: config.yaml")
	flag.BoolVar(&help, "h", false, "Get help about commands")

	// Parse global flags (before subcommands)
	flag.Parse() // print generalUsage on error

	// Show help if requested
	if help {
		generalUsage()
		return
	}

	// Create a new FlagSet for the 'set' subcommand
	setCmd := flag.NewFlagSet("set", flag.ExitOnError)
	var user, scope, dbConfig string
	var asAdmin bool

	setCmd.StringVar(&user, "u", "", "Comma-separated username and password: \"set -u <username>,<password>\"")
	setCmd.BoolVar(&asAdmin, "a", false, "Create user as admin user, used in combination with -u")
	setCmd.StringVar(&scope, "s", "", "Specify a user scope, otherwise default user config scope is used")
	setCmd.StringVar(&dbConfig, "c", "config.yaml", "Path to the config file, default: config.yaml")

	// Create context and channels for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})             // Signals server has stopped
	shutdownComplete := make(chan struct{}) // Signals shutdown process is complete
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Parse subcommand flags only if a subcommand is specified
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "set":
			err := setCmd.Parse(os.Args[2:])
			if err != nil {
				setCmd.PrintDefaults()
				os.Exit(1)
			}
			userInfo := strings.Split(user, ",")
			if len(userInfo) < 2 {
				fmt.Printf("not enough info to create user: \"set -u username,password\", only provided %v\n", userInfo)
				setCmd.PrintDefaults()
				os.Exit(1)
			}
			username := userInfo[0]
			password := userInfo[1]
			store, ok := getStore(dbConfig)
			if !ok {
				logger.Fatal("could not load db info")
			}
			user, err := store.Users.Get(username)
			if err != nil {
				newUser := users.User{
					Username: username,
					NonAdminEditable: users.NonAdminEditable{
						Password: password,
					},
				}
				for _, source := range settings.Config.Server.SourceMap {
					if source.Config.DefaultEnabled {
						newUser.Scopes = append(newUser.Scopes, users.SourceScope{
							Name:  source.Name,
							Scope: source.Config.DefaultUserScope,
						})
					}
				}

				// Create the user logic
				if asAdmin {
					logger.Info(fmt.Sprintf("Creating user as admin: %s\n", username))
				} else {
					logger.Info(fmt.Sprintf("Creating non-admin user: %s\n", username))
				}
				err = storage.CreateUser(newUser, asAdmin)
				if err != nil {
					logger.Error(fmt.Sprintf("could not create user: %v", err))
				}
				return
			}
			user.Password = password
			if asAdmin {
				user.Perm.Admin = true
			}
			err = store.Users.Save(user, true)
			if err != nil {
				logger.Error(fmt.Sprintf("could not update user: %v", err))
			}
			fmt.Printf("successfully updated user: %s\n", username)
			return

		case "version":
			fmt.Printf(`FileBrowser Quantum - A modern web-based file manager
Version        : %v
Commit         : %v
Release Info   : https://github.com/gtsteffaniak/filebrowser/releases/tag/%v
`, version.Version, version.CommitSHA, version.Version)
			return
		}
	}

	store, dbExists := getStore(configPath)
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
			logger.Fatal(fmt.Sprintf("failed to create file cache: %v", err))
		}
	} else {
		// No-op cache if no cacheDir is specified
		fileCache = diskcache.NewNoOp()
	}
	fbhttp.StartHttp(ctx, imgSvc, store, fileCache, shutdownComplete)

	return nil
}
