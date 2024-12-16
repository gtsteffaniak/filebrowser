package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
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

func cobraCmd(fn pythonFunc) cobraFunc {
	return func(cmd *cobra.Command, args []string) {
	}
}
