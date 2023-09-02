package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/settings"
)

func init() {
	configCmd.AddCommand(configInitCmd)
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
		defaults := settings.UserDefaults{}
		flags := cmd.Flags()
		getUserDefaults(flags, &defaults, true)
		_, auther := getAuthentication()
		ser := &settings.GlobalConfiguration.Server
		err := d.store.Settings.Save(&settings.GlobalConfiguration)
		checkErr(err)
		err = d.store.Settings.SaveServer(ser)
		checkErr(err)
		err = d.store.Auth.Save(auther)
		checkErr(err)

		fmt.Printf(`
Congratulations! You've set up your database to use with File Browser.
Now add your first user via 'filebrowser users add' and then you just
need to call the main command to boot up the server.
`)
		printSettings(ser, &settings.GlobalConfiguration, auther)
	}, pythonConfig{noDB: true}),
}
