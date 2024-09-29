package cmd

import (
	"crypto/tls"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"embed"

	"github.com/spf13/pflag"

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
	configPath   = "filebrowser.yaml"
)

type dirFS struct {
	http.Dir
}

func (d dirFS) Open(name string) (fs.File, error) {
	return d.Dir.Open(name)
}

func StartFilebrowser() {
	// Define the flags using pflag
	username := pflag.String("username", "", "Username for new user")
	versionCheck := pflag.Bool("version", false, "Get version information")
	password := pflag.String("password", "", "Password for new user")
	asAdmin := pflag.Bool("asAdmin", false, "Combine with username and password to create admin user")
	configFlag := pflag.StringP("config", "c", "filebrowser.yaml", "Path to the config file.")
	pflag.Parse()

	if *versionCheck {
		fmt.Println("FileBrowser Quantum - A modern web-based file manager")
		fmt.Printf("Version        : %v\n", version.Version)
		fmt.Printf("Release Info   : https://github.com/gtsteffaniak/filebrowser/releases/tag/%v\n", version.Version)
		fmt.Printf("Commit         : https://github.com/gtsteffaniak/filebrowser/commit/%v\n", version.CommitSHA)
		return
	}

	settings.Initialize(*configFlag)
	store, dbExists, err := storage.InitializeDb(settings.Config.Server.Database)
	if err != nil {
		log.Fatal("could not load db info: ", err)
	}
	if *username != "" && *password != "" {
		fmt.Println("Creating user : ", *username)
		err = storage.CreateUser(users.User{
			Username: *username,
			Password: *password,
		}, *asAdmin)
		if err != nil {
			log.Fatal("Could not create user")
		}
		return
	}
	indexingInterval := fmt.Sprint(settings.Config.Server.IndexingInterval, " minutes")
	if !settings.Config.Server.Indexing {
		indexingInterval = "disabled"
	}
	database := fmt.Sprintf("Using existing database  : %v", settings.Config.Server.Database)
	if !dbExists {
		database = fmt.Sprintf("Creating new database    : %v", settings.Config.Server.Database)
	}
	log.Printf("Initializing FileBrowser Quantum (%v)\n", version.Version)
	log.Println("Config file              :", configPath)
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
		handler, err := fbhttp.NewHandler(imgSvc, assetsFs)
		utils.CheckErr("fbhttp.NewHandler", err)
		defer listener.Close()
		log.Println("Listening on", listener.Addr().String())
		//nolint: gosec
		if err := http.Serve(listener, handler); err != nil {
			log.Fatalf("Could not start server on port %d: %v", serverConfig.Port, err)
		}
	} else {
		assetsFs := dirFS{Dir: http.Dir("frontend/dist")}
		handler, err := fbhttp.NewHandler(imgSvc, assetsFs)
		utils.CheckErr("fbhttp.NewHandler", err)
		defer listener.Close()
		log.Println("Listening on", listener.Addr().String())
		//nolint: gosec
		if err := http.Serve(listener, handler); err != nil {
			log.Fatalf("Could not start server on port %d: %v", serverConfig.Port, err)
		}
	}
	return nil
}
