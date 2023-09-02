package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/settings"
)

func init() {
	configCmd.AddCommand(configInitCmd)
	addConfigFlags(configInitCmd.Flags())
}

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
		printSettings(&s.Server, &s, auther)
	}, pythonConfig{noDB: true}),
}
