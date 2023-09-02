package cmd

import (
	"encoding/json"
	nerrors "errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/settings"
)

func init() {
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management utility",
	Long:  `Configuration management utility.`,
	Args:  cobra.NoArgs,
}

//nolint:gocyclo
func getAuthentication() (string, auth.Auther) {
	method := settings.GlobalConfiguration.Auth.Method
	var defaultAuther map[string]interface{}
	var auther auth.Auther
	if method == "proxy" {
		header := settings.GlobalConfiguration.Auth.Header
		if header == "" {
			header = defaultAuther["header"].(string)
		}

		if header == "" {
			checkErr(nerrors.New("you must set the flag 'auth.header' for method 'proxy'"))
		}

		auther = &auth.ProxyAuth{Header: header}
	}

	if method == "noauth" {
		auther = &auth.NoAuth{}
	}

	if method == "password" {
		jsonAuth := &auth.JSONAuth{}
		host := settings.GlobalConfiguration.Auth.Recaptcha.Host
		key := settings.GlobalConfiguration.Auth.Recaptcha.Key
		secret := settings.GlobalConfiguration.Auth.Recaptcha.Secret

		if key == "" {
			if kmap, ok := defaultAuther["recaptcha"].(map[string]interface{}); ok {
				key = kmap["key"].(string)
			}
		}

		if secret == "" {
			if smap, ok := defaultAuther["recaptcha"].(map[string]interface{}); ok {
				secret = smap["secret"].(string)
			}
		}

		if key != "" && secret != "" {
			jsonAuth.ReCaptcha = &auth.ReCaptcha{
				Host:   host,
				Key:    key,
				Secret: secret,
			}
		}
		auther = jsonAuth
	}

	if method == "hook" {
		command := settings.GlobalConfiguration.Auth.Command

		if command == "" {
			command = defaultAuther["command"].(string)
		}

		if command == "" {
			checkErr(nerrors.New("you must set the flag 'auth.command' for method 'hook'"))
		}

		auther = &auth.HookAuth{Command: command}
	}

	if auther == nil {
		panic(errors.ErrInvalidAuthMethod)
	}

	return method, auther
}

func printSettings(ser *settings.Server, set *settings.Settings, auther auth.Auther) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0) //nolint:gomnd

	fmt.Fprintf(w, "Sign up:\t%t\n", set.Signup)
	fmt.Fprintf(w, "Create User Dir:\t%t\n", set.CreateUserDir)
	fmt.Fprintf(w, "Auth method:\t%s\n", set.Auth.Method)
	fmt.Fprintf(w, "Shell:\t%s\t\n", strings.Join(set.Shell, " "))
	fmt.Fprintln(w, "\nFrontend:")
	fmt.Fprintf(w, "\tName:\t%s\n", set.Frontend.Name)
	fmt.Fprintf(w, "\tFiles override:\t%s\n", set.Frontend.Files)
	fmt.Fprintf(w, "\tDisable external links:\t%t\n", set.Frontend.DisableExternal)
	fmt.Fprintf(w, "\tDisable used disk percentage graph:\t%t\n", set.Frontend.DisableUsedPercentage)
	fmt.Fprintf(w, "\tColor:\t%s\n", set.Frontend.Color)
	fmt.Fprintln(w, "\nServer:")
	fmt.Fprintf(w, "\tLog:\t%s\n", ser.Log)
	fmt.Fprintf(w, "\tPort:\t%s\n", ser.Port)
	fmt.Fprintf(w, "\tBase URL:\t%s\n", ser.BaseURL)
	fmt.Fprintf(w, "\tRoot:\t%s\n", ser.Root)
	fmt.Fprintf(w, "\tSocket:\t%s\n", ser.Socket)
	fmt.Fprintf(w, "\tAddress:\t%s\n", ser.Address)
	fmt.Fprintf(w, "\tTLS Cert:\t%s\n", ser.TLSCert)
	fmt.Fprintf(w, "\tTLS Key:\t%s\n", ser.TLSKey)
	fmt.Fprintf(w, "\tExec Enabled:\t%t\n", ser.EnableExec)
	fmt.Fprintln(w, "\nDefaults:")
	fmt.Fprintf(w, "\tScope:\t%s\n", set.UserDefaults.Scope)
	fmt.Fprintf(w, "\tLocale:\t%s\n", set.UserDefaults.Locale)
	fmt.Fprintf(w, "\tView mode:\t%s\n", set.UserDefaults.ViewMode)
	fmt.Fprintf(w, "\tSingle Click:\t%t\n", set.UserDefaults.SingleClick)
	fmt.Fprintf(w, "\tCommands:\t%s\n", strings.Join(set.UserDefaults.Commands, " "))
	fmt.Fprintf(w, "\tSorting:\n")
	fmt.Fprintf(w, "\t\tBy:\t%s\n", set.UserDefaults.Sorting.By)
	fmt.Fprintf(w, "\t\tAsc:\t%t\n", set.UserDefaults.Sorting.Asc)
	fmt.Fprintf(w, "\tPermissions:\n")
	fmt.Fprintf(w, "\t\tAdmin:\t%t\n", set.UserDefaults.Perm.Admin)
	fmt.Fprintf(w, "\t\tExecute:\t%t\n", set.UserDefaults.Perm.Execute)
	fmt.Fprintf(w, "\t\tCreate:\t%t\n", set.UserDefaults.Perm.Create)
	fmt.Fprintf(w, "\t\tRename:\t%t\n", set.UserDefaults.Perm.Rename)
	fmt.Fprintf(w, "\t\tModify:\t%t\n", set.UserDefaults.Perm.Modify)
	fmt.Fprintf(w, "\t\tDelete:\t%t\n", set.UserDefaults.Perm.Delete)
	fmt.Fprintf(w, "\t\tShare:\t%t\n", set.UserDefaults.Perm.Share)
	fmt.Fprintf(w, "\t\tDownload:\t%t\n", set.UserDefaults.Perm.Download)
	w.Flush()

	b, err := json.MarshalIndent(auther, "", "  ")
	checkErr(err)
	fmt.Printf("\nAuther configuration (raw):\n\n%s\n\n", string(b))
}
