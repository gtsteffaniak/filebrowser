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
<<<<<<< HEAD
		password, err := users.HashPwd(args[1])
		checkErr(err)

=======
>>>>>>> v0.2.1
		user := &users.User{
			Username:     args[0],
			Password:     args[1],
			LockPassword: mustGetBool(cmd.Flags(), "lockPassword"),
		}
		servSettings, err := d.store.Settings.GetServer()
		checkErr(err)
		// since getUserDefaults() polluted s.Defaults.Scope
		// which makes the Scope not the one saved in the db
		// we need the right s.Defaults.Scope here
		s2, err := d.store.Settings.Get()
		checkErr(err)

		userHome, err := s2.MakeUserDir(user.Username, user.Scope, servSettings.Root)
		checkErr(err)
		user.Scope = userHome

		err = d.store.Users.Save(user)
		checkErr(err)
		printUsers([]*users.User{user})
	}, pythonConfig{}),
}
