package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/backend/storage"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

func init() {
	usersCmd.AddCommand(usersFindCmd)
	usersCmd.AddCommand(usersLsCmd)
}

var usersFindCmd = &cobra.Command{
	Use:   "find <id|username>",
	Short: "Find a user by username or id",
	Long:  `Find a user by username or id. If no flag is set, all users will be printed.`,
	Args:  cobra.ExactArgs(1),
	Run:   findUsers,
}

var usersLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all users.",
	Args:  cobra.NoArgs,
	Run:   findUsers,
}

var findUsers = cobraCmd(func(cmd *cobra.Command, args []string, store *storage.Storage) {
	var (
		list []*users.User
		user *users.User
		err  error
	)

	if len(args) == 1 {
		username, id := parseUsernameOrID(args[0])
		if username != "" {
			user, err = store.Users.Get("", username)
		} else {
			user, err = store.Users.Get("", id)
		}

		list = []*users.User{user}
	} else {
		list, err = store.Users.Gets("")
	}

	utils.CheckErr("findUsers", err)
	printUsers(list)
})
