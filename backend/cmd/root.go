package cmd

import (
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"

	"embed"

	"github.com/spf13/pflag"

	"github.com/gtsteffaniak/filebrowser/settings"
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
	storage.initializeDB(settings.Config.Server.Database)
	if *username != "" {
		fmt.Println(*username, *password)
		return
	}

	log.Printf("Initializing FileBrowser Quantum (%v) with config file: %v \n", version.Version, configPath)
	log.Println("Embeded Frontend:", !nonEmbededFS)
	err := rootCMD()
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
