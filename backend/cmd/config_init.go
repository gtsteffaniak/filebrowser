package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/settings"
)

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new database",
	Long: `Initialize a new database to use with File Browser. All of
this options can be changed in the future with the command
'filebrowser config set'. The user related flags apply
to the defaults when creating new users and you don't
override the options.`,
	Args: cobra.NoArgs,
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		auther := getAuthentication()
		s := settings.GlobalConfiguration
		s.Key = generateKey()
		err := d.store.Settings.Save(&s)
		checkErr(err)
		err = d.store.Settings.SaveServer(&s.Server)
		checkErr(err)
		err = d.store.Auth.Save(auther)
		checkErr(err)

		fmt.Printf(`
Congratulations! You've set up your database to use with File Browser.
Now add your first user via 'filebrowser users add' and then you just
need to call the main command to boot up the server.
`)
	}, pythonConfig{noDB: true}),
}

//nolint:gocyclo
func getAuthentication() auth.Auther {
	method := settings.GlobalConfiguration.Auth.Method
	var auther auth.Auther
	if method == "proxy" {
		header := settings.GlobalConfiguration.Auth.Header
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
		auther = &auth.HookAuth{Command: command}
	}

	if auther == nil {
		panic(errors.ErrInvalidAuthMethod)
	}

	return auther
}
