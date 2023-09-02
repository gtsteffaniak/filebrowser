package cmd

import (
	"crypto/tls"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"

	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/diskcache"
	fbhttp "github.com/gtsteffaniak/filebrowser/http"
	"github.com/gtsteffaniak/filebrowser/img"
	"github.com/gtsteffaniak/filebrowser/search"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/users"
)

var (
	cfgFile string
)

type dirFS struct {
	http.Dir
}

func (d dirFS) Open(name string) (fs.File, error) {
	return d.Dir.Open(name)
}

func init() {
	cobra.OnInitialize(initConfig)
	cobra.MousetrapHelpText = ""
	rootCmd.SetVersionTemplate("File Browser version {{printf \"%s\" .Version}}\n")
	settings.Initialize()

	log.Println(settings.GlobalConfiguration)
	flags := rootCmd.Flags()

	persistent := rootCmd.PersistentFlags()

	persistent.StringVarP(&cfgFile, "config", "c", "", "config file path")
	persistent.StringP("database", "d", "./filebrowser.db", "database path")
	flags.Bool("noauth", false, "use the noauth auther when using quick setup")
	flags.String("username", "admin", "username for the first user when using quick config")
	flags.String("password", "", "hashed password for the first user when using quick config (default \"admin\")")
}

var rootCmd = &cobra.Command{
	Use:   "filebrowser",
	Short: "A stylish web-based file browser",
	Long: `
If you've never run File Browser, you'll need to have a database for
it. Don't worry: you don't need to setup a separate database server.
We're using Bolt DB which is a single file database and all managed
by ourselves.

If you don't set "config", it will look for a configuration file called
filebrowser.{json, toml, yaml, yml} in the following directories:

- ./
- $HOME/
- /etc/filebrowser/

The precedence of the configuration values are as follows:

- flags
- environment variables
- configuration file
- database values
- defaults

Also, if the database path doesn't exist, File Browser will enter into
the quick setup mode and a new database will be bootstraped and a new
user created with the credentials from options "username" and "password".`,
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		serverConfig := settings.GlobalConfiguration.Server

		if !d.hadDB {
			quickSetup(cmd.Flags(), d)
		}

		workersCount := serverConfig.NumImageProcessors
		if workersCount < 1 {
			log.Fatal("Image resize workers count could not be < 1")
		}
		imgSvc := img.New(workersCount)

		var fileCache diskcache.Interface = diskcache.NewNoOp()
		cacheDir := "/tmp"
		if cacheDir != "" {
			if err := os.MkdirAll(cacheDir, 0700); err != nil { //nolint:govet,gomnd
				log.Fatalf("can't make directory %s: %s", cacheDir, err)
			}
			fileCache = diskcache.New(afero.NewOsFs(), cacheDir)
		}

		// initialize indexing and schedule indexing ever n minutes (default 5)
		go search.InitializeIndex(serverConfig.IndexingInterval)

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

func cleanupHandler(listener net.Listener, c chan os.Signal) { //nolint:interfacer
	sig := <-c
	log.Printf("Caught signal %s: shutting down.", sig)
	listener.Close()
	os.Exit(0)
}

//nolint:gocyclo
func getRunParams(flags *pflag.FlagSet, st *storage.Storage) *settings.Server {
	server, err := st.Settings.GetServer()
	checkErr(err)
	return server
}

// getParamB returns a parameter as a string and a boolean to tell if it is different from the default
//
// NOTE: we could simply bind the flags to viper and use IsSet.
// Although there is a bug on Viper that always returns true on IsSet
// if a flag is binded. Our alternative way is to manually check
// the flag and then the value from env/config/gotten by viper.
// https://github.com/spf13/viper/pull/331
func getParamB(flags *pflag.FlagSet, key string) (string, bool) {
	value, _ := flags.GetString(key)

	// If set on Flags, use it.
	if flags.Changed(key) {
		return value, true
	}

	// If set through viper (env, config), return it.
	if v.IsSet(key) {
		return v.GetString(key), true
	}

	// Otherwise use default value on flags.
	return value, false
}

func getParam(flags *pflag.FlagSet, key string) string {
	val, _ := getParamB(flags, key)
	return val
}

func setupLog(logMethod string) {
	switch logMethod {
	case "stdout":
		log.SetOutput(os.Stdout)
	case "stderr":
		log.SetOutput(os.Stderr)
	case "":
		log.SetOutput(io.Discard)
	default:
		log.SetOutput(&lumberjack.Logger{
			Filename:   logMethod,
			MaxSize:    100,
			MaxAge:     14,
			MaxBackups: 10,
		})
	}
}

func quickSetup(flags *pflag.FlagSet, d pythonData) {
	set := &settings.Settings{
		Key:              generateKey(),
		Signup:           false,
		CreateUserDir:    false,
		UserHomeBasePath: settings.DefaultUsersHomeBasePath,
		Defaults: settings.UserDefaults{
			Scope:       ".",
			Locale:      "en",
			SingleClick: false,
			Perm: users.Permissions{
				Admin:    false,
				Execute:  true,
				Create:   true,
				Rename:   true,
				Modify:   true,
				Delete:   true,
				Share:    true,
				Download: true,
			},
		},
		Frontend: settings.Frontend{},
		Commands: nil,
		Shell:    nil,
		Rules:    nil,
	}

	var err error
	if settings.GlobalConfiguration.Auth.Method == "noAuth" {
		set.Auth.Method = "noAuth"
		err = d.store.Auth.Save(&auth.NoAuth{})
	} else {
		set.Auth.Method = "json"
		err = d.store.Auth.Save(&auth.JSONAuth{})
	}
	err = d.store.Settings.Save(set)
	checkErr(err)

	ser := &settings.Server{
		BaseURL: getParam(flags, "baseurl"),
		Log:     getParam(flags, "log"),
		TLSKey:  getParam(flags, "key"),
		TLSCert: getParam(flags, "cert"),
		Root:    getParam(flags, "root"),
	}
	err = d.store.Settings.SaveServer(ser)
	checkErr(err)

	username := getParam(flags, "username")
	password := getParam(flags, "password")

	if password == "" {
		password, err = users.HashPwd("admin")
		checkErr(err)
	}

	if username == "" || password == "" {
		log.Fatal("username and password cannot be empty during quick setup")
	}

	user := &users.User{
		Username:     username,
		Password:     password,
		LockPassword: false,
	}

	set.Defaults.Apply(user)
	user.Perm.Admin = true

	err = d.store.Users.Save(user)
	checkErr(err)
}

func initConfig() {
	if cfgFile == "" {
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/filebrowser/")
		v.SetConfigName("filebrowser")
	} else {
		v.SetConfigFile(cfgFile)
	}

	v.SetEnvPrefix("FB")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(v.ConfigParseError); ok {
			panic(err)
		}
		cfgFile = "No config file used"
	} else {
		cfgFile = "Using config file: " + v.ConfigFileUsed()
	}
}
