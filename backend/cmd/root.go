package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"

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

func getStore(config string) (*storage.Storage, bool) {
	// Use the config file (global flag)
	settings.Initialize(config)
	store, hasDB, err := storage.InitializeDb(settings.Config.Server.Database)
	if err != nil {
		logger.Fatal(fmt.Sprintf("could not load db info: %v", err))
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

	// Parse subcommand flags only if a subcommand is specified
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "set":
			err := setCmd.Parse(os.Args)
			if err != nil {
				setCmd.PrintDefaults()
				os.Exit(1)
			}
			userInfo := strings.Split(user, ",")
			if len(userInfo) < 2 {
				fmt.Println("not enough info to create user: \"set -u username,password\"")
				setCmd.PrintDefaults()
				os.Exit(1)
			}
			username := userInfo[0]
			password := userInfo[1]
			getStore(dbConfig)
			// Create the user logic
			if asAdmin {
				logger.Info(fmt.Sprintf("Creating user as admin: %s\n", username))
			} else {
				logger.Info(fmt.Sprintf("Creating non-admin user: %s\n", username))
			}
			newUser := users.User{
				Username: username,
				Password: password,
			}
			if scope != "" {
				newUser.Scope = scope
			}
			err = storage.CreateUser(newUser, asAdmin)
			if err != nil {
				logger.Fatal(fmt.Sprintf("could not create user: %v", err))
			}
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
	sources := []string{}
	for _, v := range settings.Config.Server.Sources {
		sources = append(sources, v.Name+": "+v.Path)
	}
	logger.Info(fmt.Sprintf("Initializing FileBrowser Quantum (%v)", version.Version))
	logger.Info(fmt.Sprintf("Using Config file        : %v", configPath))
	logger.Debug(fmt.Sprintf("Embeded frontend         : %v", os.Getenv("FILEBROWSER_NO_EMBEDED") != "true"))
	logger.Info(database)
	logger.Info(fmt.Sprintf("Sources                  : %v", sources))
	serverConfig := settings.Config.Server
	swagInfo := docs.SwaggerInfo
	swagInfo.BasePath = serverConfig.BaseURL
	swag.Register(docs.SwaggerInfo.InstanceName(), swagInfo)
	// initialize indexing and schedule indexing ever n minutes (default 5)
	sourceConfigs := settings.Config.Server.Sources
	if len(sourceConfigs) == 0 {
		logger.Fatal("No sources configured, exiting...")
	}
	for _, source := range sourceConfigs {
		go files.Initialize(source)
	}
	if err := rootCMD(store, &serverConfig); err != nil {
		logger.Fatal(fmt.Sprintf("Error starting filebrowser: %v", err))
	}
}

func rootCMD(store *storage.Storage, serverConfig *settings.Server) error {
	if serverConfig.NumImageProcessors < 1 {
		logger.Fatal("Image resize workers count could not be < 1")
	}
	imgSvc := img.New(serverConfig.NumImageProcessors)

	cacheDir := "/tmp"
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
	fbhttp.StartHttp(imgSvc, store, fileCache)

	return nil
}
