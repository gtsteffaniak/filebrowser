package cmd

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"embed"

	"github.com/gtsteffaniak/filebrowser/diskcache"
	"github.com/gtsteffaniak/filebrowser/files"
	fbhttp "github.com/gtsteffaniak/filebrowser/http"
	"github.com/gtsteffaniak/filebrowser/img"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/users"
	"github.com/gtsteffaniak/filebrowser/utils"
	"github.com/gtsteffaniak/filebrowser/version"
)

//go:embed dist/*
var assets embed.FS

var (
	nonEmbededFS = os.Getenv("FILEBROWSER_NO_EMBEDED") == "true"
)

type dirFS struct {
	http.Dir
}

func (d dirFS) Open(name string) (fs.File, error) {
	return d.Dir.Open(name)
}

func getStore(config string) (*storage.Storage, bool) {
	// Use the config file (global flag)
	log.Printf("Using Config file        : %v", config)
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
	flag.StringVar(&configPath, "c", "filebrowser.yaml", "Path to the config file.")
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
	setCmd.StringVar(&dbConfig, "c", "filebrowser.yaml", "Path to the config file.")

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
	indexingInterval := fmt.Sprint(settings.Config.Server.IndexingInterval, " minutes")
	if !settings.Config.Server.Indexing {
		indexingInterval = "disabled"
	}
	database := fmt.Sprintf("Using existing database  : %v", settings.Config.Server.Database)
	if !dbExists {
		database = fmt.Sprintf("Creating new database    : %v", settings.Config.Server.Database)
	}
	log.Printf("Initializing FileBrowser Quantum (%v)\n", version.Version)
	log.Println("Embeded frontend         :", !nonEmbededFS)
	log.Println(database)
	log.Println("Sources                  :", settings.Config.Server.Root)
	log.Print("Indexing interval        : ", indexingInterval)

	serverConfig := settings.Config.Server
	// initialize indexing and schedule indexing ever n minutes (default 5)
	go files.InitializeIndex(serverConfig.IndexingInterval, serverConfig.Indexing)
	if err := rootCMD(store, &serverConfig); err != nil {
		log.Fatal("Error starting filebrowser:", err)
	}
}

func cleanupHandler(listener net.Listener, c chan os.Signal) { //nolint:interfacer
	sig := <-c
	log.Printf("Caught signal %s: shutting down.", sig)
	listener.Close()
	os.Exit(0)
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

	fbhttp.SetupEnv(store, serverConfig, fileCache)

	_, err := os.Stat(serverConfig.Root)
	utils.CheckErr(fmt.Sprint("cmd os.Stat ", serverConfig.Root), err)
	var listener net.Listener
	address := serverConfig.Address + ":" + strconv.Itoa(serverConfig.Port)
	switch {
	case serverConfig.Socket != "":
		listener, err = net.Listen("unix", serverConfig.Socket)
		utils.CheckErr("net.Listen", err)
		err = os.Chmod(serverConfig.Socket, os.FileMode(0666)) // socket-perm
		utils.CheckErr("os.Chmod", err)
	case serverConfig.TLSKey != "" && serverConfig.TLSCert != "":
		cer, err := tls.LoadX509KeyPair(serverConfig.TLSCert, serverConfig.TLSKey) //nolint:govet
		utils.CheckErr("tls.LoadX509KeyPair", err)
		listener, err = tls.Listen("tcp", address, &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cer}},
		)
		utils.CheckErr("tls.Listen", err)
	default:
		listener, err = net.Listen("tcp", address)
		utils.CheckErr("net.Listen", err)
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go cleanupHandler(listener, sigc)
	if !nonEmbededFS {
		assetsFs, err := fs.Sub(assets, "dist")
		if err != nil {
			log.Fatal("Could not embed frontend. Does backend/cmd/dist exist? Must be built and exist first")
		}
		fbhttp.Setup(imgSvc, assetsFs)
	} else {
		assetsFs := dirFS{Dir: http.Dir("frontend/dist")}
		fbhttp.Setup(imgSvc, assetsFs)
	}
	return nil
}
