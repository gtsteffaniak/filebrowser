package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/users"
)

func init() {
	usersCmd.AddCommand(usersAddCmd)
}

var usersAddCmd = &cobra.Command{
	Use:   "add <username> <password>",
	Short: "Create a new user",
	Long:  `Create a new user and add it to the database.`,
	Args:  cobra.ExactArgs(2), //nolint:gomnd
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		user := &users.User{
			Username:     args[0],
			Password:     args[1],
			LockPassword: mustGetBool(cmd.Flags(), "lockPassword"),
		}
		servSettings, err := d.store.Settings.GetServer()
		checkErr("d.store.Settings.GetServer()", err)
		// since getUserDefaults() polluted s.Defaults.Scope
		// which makes the Scope not the one saved in the db
		// we need the right s.Defaults.Scope here
		s2, err := d.store.Settings.Get()
		checkErr("d.store.Settings.Get()", err)

		userHome, err := s2.MakeUserDir(user.Username, user.Scope, servSettings.Root)
		checkErr("s2.MakeUserDir", err)
		user.Scope = userHome

		err = d.store.Users.Save(user)
		checkErr("d.store.Users.Save", err)
		printUsers([]*users.User{user})
	}, pythonConfig{}),
}
