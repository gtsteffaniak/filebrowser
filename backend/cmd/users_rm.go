package cmd

import (
	"log"

	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/spf13/cobra"
)

func init() {
	usersCmd.AddCommand(usersRmCmd)
}

var usersRmCmd = &cobra.Command{
	Use:   "rm <id|username>",
	Short: "Delete a user by username or id",
	Long:  `Delete a user by username or id`,
	Args:  cobra.ExactArgs(1),
	Run: python(func(cmd *cobra.Command, args []string, store *storage.Storage) {
		username, id := parseUsernameOrID(args[0])
		var err error

		if username != "" {
			err = d.store.Users.Delete(username)
		} else {
			err = d.store.Users.Delete(id)
		}

		checkErr("usersRmCmd", err)
		log.Println("user deleted successfully")
	}, pythonConfig{}),
}
