package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

func init() {
	usersCmd.AddCommand(usersUpdateCmd)
}

var usersUpdateCmd = &cobra.Command{
	Use:   "update <id|username>",
	Short: "Updates an existing user",
	Long: `Updates an existing user. Set the flags for the
options you want to change.`,
	Args: cobra.ExactArgs(1),
	Run: cobraCmd(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		username, id := parseUsernameOrID(args[0])

		var (
			err  error
			user *users.User
		)

		if id != 0 {
			user, err = store.Users.Get("", id)
		} else {
			user, err = store.Users.Get("", username)
		}
		utils.CheckErr("store.Users.Get", err)

		err = store.Users.Update(user)
		utils.CheckErr("store.Users.Update", err)
		printUsers([]*users.User{user})
	}),
}
