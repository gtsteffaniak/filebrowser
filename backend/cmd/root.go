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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gtsteffaniak/filebrowser/diskcache"
	"github.com/gtsteffaniak/filebrowser/files"
	fbhttp "github.com/gtsteffaniak/filebrowser/http"
	"github.com/gtsteffaniak/filebrowser/img"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
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
	username := pflag.String("user", "", "Username")
	password := pflag.String("password", "", "Password")
	configFlag := pflag.StringP("config", "c", "filebrowser.yaml", "Path to the config file.")

	// Parse the flags using pflag
	pflag.Parse()

	settings.Initialize(*configFlag)
	storage.InitializeDb(settings.Config.Server.Database)
	if *username != "" {
		fmt.Println(*username, *password)
		return
	}

	log.Printf("Initializing FileBrowser Quantum (%v) with config file: %v \n", version.Version, configPath)
	log.Println("Embeded Frontend:", !nonEmbededFS)
	err := rootCmd
	if err != nil {
		log.Fatal("Error starting filebrowser:", err)
	}

}

func cleanupHandler(listener net.Listener, c chan os.Signal) { //nolint:interfacer
	sig := <-c
	log.Printf("Caught signal %s: shutting down.", sig)
	listener.Close()
	os.Exit(0)
}

var rootCmd = &cobra.Command{
	Use: "filebrowser",
	Run: cobraCmd(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		serverConfig := settings.Config.Server
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
		// initialize indexing and schedule indexing ever n minutes (default 5)
		go files.InitializeIndex(serverConfig.IndexingInterval, serverConfig.Indexing)
		_, err := os.Stat(serverConfig.Root)
		utils.CheckErr(fmt.Sprint("cmd os.Stat ", serverConfig.Root), err)
		var listener net.Listener
		address := serverConfig.Address + ":" + strconv.Itoa(serverConfig.Port)
		switch {
		case serverConfig.Socket != "":
			listener, err = net.Listen("unix", serverConfig.Socket)
			utils.CheckErr("net.Listen", err)
			socketPerm, err := cmd.Flags().GetUint32("socket-perm") //nolint:govet
			utils.CheckErr("cmd.Flags().GetUint32", err)
			err = os.Chmod(serverConfig.Socket, os.FileMode(socketPerm))
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
			handler, err := fbhttp.NewHandler(imgSvc, fileCache, store, &serverConfig, assetsFs)
			utils.CheckErr("fbhttp.NewHandler", err)
			defer listener.Close()
			log.Println("Listening on", listener.Addr().String())
			//nolint: gosec
			if err := http.Serve(listener, handler); err != nil {
				log.Fatalf("Could not start server on port %d: %v", serverConfig.Port, err)
			}
		} else {
			assetsFs := dirFS{Dir: http.Dir("frontend/dist")}
			handler, err := fbhttp.NewHandler(imgSvc, fileCache, store, &serverConfig, assetsFs)
			utils.CheckErr("fbhttp.NewHandler", err)
			defer listener.Close()
			log.Println("Listening on", listener.Addr().String())
			//nolint: gosec
			if err := http.Serve(listener, handler); err != nil {
				log.Fatalf("Could not start server on port %d: %v", serverConfig.Port, err)
			}
		}

	}, pythonConfig{allowNoDB: true}),
}
