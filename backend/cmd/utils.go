package cmd

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gtsteffaniak/filebrowser/diskcache"
	"github.com/gtsteffaniak/filebrowser/files"
	fbhttp "github.com/gtsteffaniak/filebrowser/http"
	"github.com/gtsteffaniak/filebrowser/img"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/utils"
)

func mustGetString(flags *pflag.FlagSet, flag string) string {
	s, err := flags.GetString(flag)
	utils.CheckErr("mustGetString", err)
	return s
}

func mustGetBool(flags *pflag.FlagSet, flag string) bool {
	b, err := flags.GetBool(flag)
	utils.CheckErr("mustGetBool", err)
	return b
}

func mustGetUint(flags *pflag.FlagSet, flag string) uint {
	b, err := flags.GetUint(flag)
	utils.CheckErr("mustGetUint", err)
	return b
}

type cobraFunc func(cmd *cobra.Command, args []string)
type pythonFunc func(cmd *cobra.Command, args []string, store *storage.Storage)

type pythonConfig struct {
	noDB      bool
	allowNoDB bool
}

func rootCMD() error {
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
		handler, err := fbhttp.NewHandler(imgSvc, fileCache, d.store, &serverConfig, assetsFs)
		utils.CheckErr("fbhttp.NewHandler", err)
		defer listener.Close()
		log.Println("Listening on", listener.Addr().String())
		//nolint: gosec
		if err := http.Serve(listener, handler); err != nil {
			log.Fatalf("Could not start server on port %d: %v", serverConfig.Port, err)
		}
	} else {
		assetsFs := dirFS{Dir: http.Dir("frontend/dist")}
		handler, err := fbhttp.NewHandler(imgSvc, fileCache, d.store, &serverConfig, assetsFs)
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

func marshal(filename string, data interface{}) error {
	fd, err := os.Create(filename)

	utils.CheckErr("os.Create", err)
	defer fd.Close()

	switch ext := filepath.Ext(filename); ext {
	case ".json":
		encoder := json.NewEncoder(fd)
		encoder.SetIndent("", "    ")
		return encoder.Encode(data)
	case ".yml", ".yaml": //nolint:goconst
		_, err := yaml.Marshal(fd)
		return err
	default:
		return errors.New("invalid format: " + ext)
	}
}

func unmarshal(filename string, data interface{}) error {
	fd, err := os.Open(filename)
	utils.CheckErr("os.Open", err)
	defer fd.Close()

	switch ext := filepath.Ext(filename); ext {
	case ".json":
		return json.NewDecoder(fd).Decode(data)
	case ".yml", ".yaml":
		return yaml.NewDecoder(fd).Decode(data)
	default:
		return errors.New("invalid format: " + ext)
	}
}

func jsonYamlArg(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(1)(cmd, args); err != nil {
		return err
	}

	switch ext := filepath.Ext(args[0]); ext {
	case ".json", ".yml", ".yaml":
		return nil
	default:
		return errors.New("invalid format: " + ext)
	}
}
