package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/asdine/storm"
	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/storage/bolt"
)

func checkErr(source string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", source, err)
	}
}

func mustGetString(flags *pflag.FlagSet, flag string) string {
	s, err := flags.GetString(flag)
	checkErr("mustGetString", err)
	return s
}

func mustGetBool(flags *pflag.FlagSet, flag string) bool {
	b, err := flags.GetBool(flag)
	checkErr("mustGetBool", err)
	return b
}

func mustGetUint(flags *pflag.FlagSet, flag string) uint {
	b, err := flags.GetUint(flag)
	checkErr("mustGetUint", err)
	return b
}

func generateKey() []byte {
	k, err := settings.GenerateKey()
	checkErr("generateKey", err)
	return k
}

type cobraFunc func(cmd *cobra.Command, args []string)
type pythonFunc func(cmd *cobra.Command, args []string, store *storage.Storage)

type pythonConfig struct {
	noDB      bool
	allowNoDB bool
}

func dbExists(path string) (bool, error) {
	stat, err := os.Stat(path)
	if err == nil {
		return stat.Size() != 0, nil
	}

	if os.IsNotExist(err) {
		d := filepath.Dir(path)
		_, err = os.Stat(d)
		if os.IsNotExist(err) {
			if err := os.MkdirAll(d, 0700); err != nil { //nolint:govet,gomnd
				return false, err
			}
			return false, nil
		}
	}

	return false, err
}

func initDb() {
	path := settings.Config.Server.Database
	exists, err := dbExists(path)

	if !exists {
		quickSetup(d)
	}

	if err != nil {
		panic(err)
	} else if exists && cfg.noDB {
		log.Fatal(path + " already exists")
	} else if !exists && !cfg.noDB && !cfg.allowNoDB {
		log.Fatal(path + " does not exist. Please run 'filebrowser config init' first.")
	}

	data.hadDB = exists
	db, err := storm.Open(path)
	checkErr(fmt.Sprintf("storm.Open path %v", path), err)

	defer db.Close()
	data.store, err = bolt.NewStorage(db)
	checkErr("bolt.NewStorage", err)
	fn(cmd, args, data)
}

func marshal(filename string, data interface{}) error {
	fd, err := os.Create(filename)

	checkErr("os.Create", err)
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
	checkErr("os.Open", err)
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
