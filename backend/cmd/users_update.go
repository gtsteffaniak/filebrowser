package cmd

import (
	"github.com/spf13/cobra"

	"github.com/gtsteffaniak/filebrowser/users"
)

func init() {
	usersCmd.AddCommand(usersUpdateCmd)

	usersUpdateCmd.Flags().StringP("password", "p", "", "new password")
	usersUpdateCmd.Flags().StringP("username", "u", "", "new username")
}

var usersUpdateCmd = &cobra.Command{
	Use:   "update <id|username>",
	Short: "Updates an existing user",
	Long: `Updates an existing user. Set the flags for the
options you want to change.`,
	Args: cobra.ExactArgs(1),
	Run: python(func(cmd *cobra.Command, args []string, d pythonData) {
		username, id := parseUsernameOrID(args[0])
		flags := cmd.Flags()
		password := mustGetString(flags, "password")
		newUsername := mustGetString(flags, "username")

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
		user.Scope = user.Scope
		user.Locale = user.Locale
		user.ViewMode = user.ViewMode
		user.SingleClick = user.SingleClick
		user.Perm = user.Perm
		user.Commands = user.Commands
		user.Sorting = user.Sorting
		user.LockPassword = user.LockPassword

		if newUsername != "" {
			user.Username = newUsername
		}

		if password != "" {
			user.Password, err = users.HashPwd(password)
			checkErr(err)
		}

		err = d.store.Users.Update(user)
		checkErr(err)
		printUsers([]*users.User{user})
	}, pythonConfig{}),
}
