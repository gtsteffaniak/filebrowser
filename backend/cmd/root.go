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
	"syscall"

	"embed"

	"github.com/spf13/pflag"

	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/diskcache"
	"github.com/gtsteffaniak/filebrowser/files"
	fbhttp "github.com/gtsteffaniak/filebrowser/http"
	"github.com/gtsteffaniak/filebrowser/img"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
	"github.com/gtsteffaniak/filebrowser/version"
)

//go:embed dist/*
var assets embed.FS

var nonEmbededFS = os.Getenv("FILEBROWSER_NO_EMBEDED") == "true"

type dirFS struct {
	http.Dir
}

func (d dirFS) Open(name string) (fs.File, error) {
	return d.Dir.Open(name)
}

func init() {
	// Define a flag for the config option (-c or --config)
	configFlag := pflag.StringP("config", "c", "filebrowser.yaml", "Path to the config file")
	// Bind the flags to the pflag command line parser
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	log.Printf("Initializing Filebrowser Quantum (%v) with config file: %v \n", version.Version, *configFlag)
	log.Println("Embeded Frontend:", !nonEmbededFS)
	settings.Initialize(*configFlag)
}

var rootCmd = &cobra.Command{
	Use: "filebrowser",
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		serverConfig := settings.Config.Server
		if !d.hadDB {
			quickSetup(d)
		}
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
		checkErr(fmt.Sprint("cmd os.Stat ", serverConfig.Root), err)
		var listener net.Listener
		address := serverConfig.Address + ":" + strconv.Itoa(serverConfig.Port)
		switch {
		case serverConfig.Socket != "":
			listener, err = net.Listen("unix", serverConfig.Socket)
			checkErr("net.Listen", err)
			socketPerm, err := cmd.Flags().GetUint32("socket-perm") //nolint:govet
			checkErr("cmd.Flags().GetUint32", err)
			err = os.Chmod(serverConfig.Socket, os.FileMode(socketPerm))
			checkErr("os.Chmod", err)
		case serverConfig.TLSKey != "" && serverConfig.TLSCert != "":
			cer, err := tls.LoadX509KeyPair(serverConfig.TLSCert, serverConfig.TLSKey) //nolint:govet
			checkErr("tls.LoadX509KeyPair", err)
			listener, err = tls.Listen("tcp", address, &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cer}},
			)
			checkErr("tls.Listen", err)
		default:
			listener, err = net.Listen("tcp", address)
			checkErr("net.Listen", err)
		}
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
		go cleanupHandler(listener, sigc)
		if !nonEmbededFS {
			assetsFs, err := fs.Sub(assets, "dist")
			if err != nil {
				log.Fatal("Could not embed frontend. Does backend/cmd/dist exist? Must be built and exist first")
			}
			handler, err := fbhttp.NewHandler(imgSvc, fileCache, d.store, &serverConfig, assetsFs)
			checkErr("fbhttp.NewHandler", err)
			defer listener.Close()
			log.Println("Listening on", listener.Addr().String())
			//nolint: gosec
			if err := http.Serve(listener, handler); err != nil {
				log.Fatalf("Could not start server on port %d: %v", serverConfig.Port, err)
			}
		} else {
			assetsFs := dirFS{Dir: http.Dir("frontend/dist")}
			handler, err := fbhttp.NewHandler(imgSvc, fileCache, d.store, &serverConfig, assetsFs)
			checkErr("fbhttp.NewHandler", err)
			defer listener.Close()
			log.Println("Listening on", listener.Addr().String())
			//nolint: gosec
			if err := http.Serve(listener, handler); err != nil {
				log.Fatalf("Could not start server on port %d: %v", serverConfig.Port, err)
			}
		}

	}, pythonConfig{allowNoDB: true}),
}

func StartFilebrowser() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal("Error starting filebrowser:", err)
	}
}

func cleanupHandler(listener net.Listener, c chan os.Signal) { //nolint:interfacer
	sig := <-c
	log.Printf("Caught signal %s: shutting down.", sig)
	listener.Close()
	os.Exit(0)
}

func quickSetup(d pythonData) {
	settings.Config.Auth.Key = generateKey()
	if settings.Config.Auth.Method == "noauth" {
		err := d.store.Auth.Save(&auth.NoAuth{})
		checkErr("d.store.Auth.Save", err)
	} else {
		settings.Config.Auth.Method = "password"
		err := d.store.Auth.Save(&auth.JSONAuth{})
		checkErr("d.store.Auth.Save", err)
	}
	err := d.store.Settings.Save(&settings.Config)
	checkErr("d.store.Settings.Save", err)
	err = d.store.Settings.SaveServer(&settings.Config.Server)
	checkErr("d.store.Settings.SaveServer", err)
	user := users.ApplyDefaults(users.User{})
	user.Username = settings.Config.Auth.AdminUsername
	user.Password = settings.Config.Auth.AdminPassword
	user.Perm.Admin = true
	user.Scope = "./"
	user.DarkMode = true
	user.ViewMode = "normal"
	user.LockPassword = false
	user.Perm = settings.Permissions{
		Create:   true,
		Rename:   true,
		Modify:   true,
		Delete:   true,
		Share:    true,
		Download: true,
		Admin:    true,
	}
	err = d.store.Users.Save(&user)
	checkErr("d.store.Users.Save", err)
}
