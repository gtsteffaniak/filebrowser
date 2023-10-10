package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/users"
)

func init() {
	usersCmd.AddCommand(usersUpdateCmd)
<<<<<<< HEAD

	usersUpdateCmd.Flags().StringP("password", "p", "", "new password")
	usersUpdateCmd.Flags().StringP("username", "u", "", "new username")
=======
>>>>>>> v0.2.1
}

var usersUpdateCmd = &cobra.Command{
	Use:   "update <id|username>",
	Short: "Updates an existing user",
	Long: `Updates an existing user. Set the flags for the
options you want to change.`,
	Args: cobra.ExactArgs(1),
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		username, id := parseUsernameOrID(args[0])

		var (
			err  error
			user *users.User
		)

		if id != 0 {
			user, err = d.store.Users.Get("", id)
		} else {
			user, err = d.store.Users.Get("", username)
		}
		checkErr(err)

		err = d.store.Users.Update(user)
		checkErr(err)
		printUsers([]*users.User{user})
	}, pythonConfig{}),
}
