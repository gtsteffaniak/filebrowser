package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

func init() {
	usersCmd.AddCommand(usersAddCmd)
}

var usersAddCmd = &cobra.Command{
	Use:   "add <username> <password>",
	Short: "Create a new user",
	Long:  `Create a new user and add it to the database.`,
	Args:  cobra.ExactArgs(2), //nolint:gomnd
	Run: cobraCmd(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		user := &users.User{
			Username:     args[0],
			Password:     args[1],
			LockPassword: mustGetBool(cmd.Flags(), "lockPassword"),
		}
		servSettings, err := store.Settings.GetServer()
		utils.CheckErr("store.Settings.GetServer()", err)
		// since getUserDefaults() polluted s.Defaults.Scope
		// which makes the Scope not the one saved in the db
		// we need the right s.Defaults.Scope here
		s2, err := store.Settings.Get()
		utils.CheckErr("store.Settings.Get()", err)

		userHome, err := s2.MakeUserDir(user.Username, user.Scope, servSettings.Root)
		utils.CheckErr("s2.MakeUserDir", err)
		user.Scope = userHome

		err = store.Users.Save(user)
		utils.CheckErr("store.Users.Save", err)
		printUsers([]*users.User{user})
	}),
}
