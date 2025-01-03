package cmd

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/diskcache"
	"github.com/gtsteffaniak/filebrowser/backend/files"
	fbhttp "github.com/gtsteffaniak/filebrowser/backend/http"
	"github.com/gtsteffaniak/filebrowser/backend/img"
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
		log.Fatal("could not load db info: ", err)
	}
	return store, hasDB
}

func generalUsage() {
	fmt.Printf(`usage: ./html-web-crawler <command> [options] --urls <urls>
  commands:
    collect  Collect data from URLs
    crawl    Crawl URLs and collect data
    install  Install chrome browser for javascript enabled scraping.
               Note: Consider instead to install via native package manager,
                     then set "CHROME_EXECUTABLE" in the environment
	` + "\n")
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
				log.Printf("Creating user as admin: %s\n", username)
			} else {
				log.Printf("Creating user: %s\n", username)
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
				log.Fatal("Could not create user: ", err)
			}
			return
		case "version":
			fmt.Println("FileBrowser Quantum - A modern web-based file manager")
			fmt.Printf("Version        : %v\n", version.Version)
			fmt.Printf("Commit         : %v\n", version.CommitSHA)
			fmt.Printf("Release Info   : https://github.com/gtsteffaniak/filebrowser/releases/tag/%v\n", version.Version)
			return
		}
	}
	store, dbExists := getStore(configPath)
	database := fmt.Sprintf("Using existing database  : %v", settings.Config.Server.Database)
	if !dbExists {
		database = fmt.Sprintf("Creating new database    : %v", settings.Config.Server.Database)
	}
	log.Printf("Initializing FileBrowser Quantum (%v)\n", version.Version)
	log.Printf("Using Config file        : %v", configPath)
	log.Println("Embeded frontend         :", os.Getenv("FILEBROWSER_NO_EMBEDED") != "true")
	log.Println(database)
	sources := []string{}
	for name := range settings.Config.Server.Sources {
		fmt.Println("name", name, settings.Config.Server.Sources)
		sources = append(sources, name)
	}
	log.Println("Sources                  :", sources)

	serverConfig := settings.Config.Server
	swagInfo := docs.SwaggerInfo
	swagInfo.BasePath = serverConfig.BaseURL
	swag.Register(docs.SwaggerInfo.InstanceName(), swagInfo)
	// initialize indexing and schedule indexing ever n minutes (default 5)
	sourceConfigs := settings.Config.Server.Sources
	if len(sourceConfigs) == 0 {
		log.Fatal("No sources configured, exiting...")
	}
	for _, source := range sourceConfigs {
		fmt.Println("indexing source", source)
		go files.Initialize(source)
	}
	if err := rootCMD(store, &serverConfig); err != nil {
		log.Fatal("Error starting filebrowser:", err)
	}
}

func rootCMD(store *storage.Storage, serverConfig *settings.Server) error {
	if serverConfig.NumImageProcessors < 1 {
		log.Fatal("Image resize workers count could not be < 1")
	}
	imgSvc := img.New(serverConfig.NumImageProcessors)

	cacheDir := "/tmp"
	var fileCache diskcache.Interface

	// Use file cache if cacheDir is specified
	if cacheDir != "" {
		var err error
		fileCache, err = diskcache.NewFileCache(cacheDir)
		if err != nil {
			log.Fatalf("failed to create file cache: %v", err)
		}
	} else {
		// No-op cache if no cacheDir is specified
		fileCache = diskcache.NewNoOp()
	}
	fbhttp.StartHttp(imgSvc, store, fileCache)

	return nil
}
