package cmd

import (
	"crypto/tls"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/spf13/pflag"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/diskcache"
	fbhttp "github.com/gtsteffaniak/filebrowser/http"
	"github.com/gtsteffaniak/filebrowser/img"
	"github.com/gtsteffaniak/filebrowser/index"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/users"
)

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
	log.Println("Initializing with config file:", *configFlag)
	settings.Initialize(*configFlag)
}

var rootCmd = &cobra.Command{
	Use: "filebrowser",
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		serverConfig := settings.GlobalConfiguration.Server
		if !d.hadDB {
			quickSetup(d)
		}
		if serverConfig.NumImageProcessors < 1 {
			log.Fatal("Image resize workers count could not be < 1")
		}
		imgSvc := img.New(serverConfig.NumImageProcessors)
		var fileCache diskcache.Interface = diskcache.NewNoOp()
		cacheDir := "/tmp"
		if cacheDir != "" {
			if err := os.MkdirAll(cacheDir, 0700); err != nil { //nolint:govet,gomnd
				log.Fatalf("can't make directory %s: %s", cacheDir, err)
			}
			fileCache = diskcache.New(afero.NewOsFs(), cacheDir)
		}
		// initialize indexing and schedule indexing ever n minutes (default 5)
		go index.Initialize(serverConfig.IndexingInterval)
		_, err := os.Stat(serverConfig.Root)
		checkErr(err)
		var listener net.Listener
		address := serverConfig.Address + ":" + strconv.Itoa(serverConfig.Port)
		switch {
		case serverConfig.Socket != "":
			listener, err = net.Listen("unix", serverConfig.Socket)
			checkErr(err)
			socketPerm, err := cmd.Flags().GetUint32("socket-perm") //nolint:govet
			checkErr(err)
			err = os.Chmod(serverConfig.Socket, os.FileMode(socketPerm))
			checkErr(err)
		case serverConfig.TLSKey != "" && serverConfig.TLSCert != "":
			cer, err := tls.LoadX509KeyPair(serverConfig.TLSCert, serverConfig.TLSKey) //nolint:govet
			checkErr(err)
			listener, err = tls.Listen("tcp", address, &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cer}},
			)
			checkErr(err)
		default:
			listener, err = net.Listen("tcp", address)
			checkErr(err)
		}
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
		go cleanupHandler(listener, sigc)
		assetsFs := dirFS{Dir: http.Dir("frontend/dist")}
		handler, err := fbhttp.NewHandler(imgSvc, fileCache, d.store, &serverConfig, assetsFs)
		checkErr(err)
		defer listener.Close()
		log.Println("Listening on", listener.Addr().String())
		//nolint: gosec
		if err := http.Serve(listener, handler); err != nil {
			log.Fatal(err)
		}
	}, pythonConfig{allowNoDB: true}),
}

func StartFilebrowser() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func cleanupHandler(listener net.Listener, c chan os.Signal) { //nolint:interfacer
	sig := <-c
	log.Printf("Caught signal %s: shutting down.", sig)
	listener.Close()
	os.Exit(0)
}

func quickSetup(d pythonData) {
	settings.GlobalConfiguration.Auth.Key = generateKey()
	if settings.GlobalConfiguration.Auth.Method == "noauth" {
		err := d.store.Auth.Save(&auth.NoAuth{})
		checkErr(err)
	} else {
		settings.GlobalConfiguration.Auth.Method = "password"
		err := d.store.Auth.Save(&auth.JSONAuth{})
		checkErr(err)
	}
	err := d.store.Settings.Save(&settings.GlobalConfiguration)
	checkErr(err)
	err = d.store.Settings.SaveServer(&settings.GlobalConfiguration.Server)
	checkErr(err)
	user := &users.User{}
	settings.GlobalConfiguration.UserDefaults.Apply(user)
	user.Username = settings.GlobalConfiguration.Auth.AdminUsername
	user.Password = settings.GlobalConfiguration.Auth.AdminPassword
	user.Perm.Admin = true
	user.Scope = "./"
	user.DarkMode = true
	user.ViewMode = "normal"
	user.LockPassword = false
	user.Perm = users.Permissions{
		Create:   true,
		Rename:   true,
		Modify:   true,
		Delete:   true,
		Share:    true,
		Download: true,
		Admin:    true,
	}
	err = d.store.Users.Save(user)
	checkErr(err)
}
